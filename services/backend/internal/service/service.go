// Package service configures the API server, its routes, and the business logic that spans more
// than one entity.
package service

import (
	"net/http"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

// Config holds the deployment-specific values the API reports or enforces. Everything here is
// resolved by the caller (see cmd/backend) rather than read from the environment in-place.
type Config struct {
	// Environment is the deployment name reported by /debug (e.g. stag, prod).
	Environment string
	// Commit is the git SHA the running binary was built from.
	Commit string
	// AllowedOrigin is the frontend origin permitted to call this API from a browser. Empty
	// disables CORS headers entirely.
	AllowedOrigin string
}

// Service owns the API's dependencies and serves its routes.
type Service struct {
	cfg      Config
	blogs    repository.BlogRepository
	users    repository.UserRepository
	verifier repository.TokenVerifier
}

func New(cfg Config, blogs repository.BlogRepository, users repository.UserRepository, verifier repository.TokenVerifier) *Service {
	return &Service{cfg: cfg, blogs: blogs, users: users, verifier: verifier}
}

// Handler returns the fully-wired API, ready to serve.
func (s *Service) Handler() http.Handler {
	// Every write requires a verified caller, since it always resolves to a specific owner.
	authed := func(h http.HandlerFunc) http.Handler {
		return requireAuth(s.verifier, h)
	}
	// GET /blogs, GET /blogs/{slug}, and GET /users/{username} admit anonymous callers. Blog visibility is
	// enforced downstream: entity.Blog.CanBeReadBy already treats the zero Caller as seeing only
	// public posts, so no handler change was needed to support this - only relaxing which requests
	// reach it. A profile has nothing caller-specific to hide, so GetUser needed no change either -
	// a display name and bio are meant to be public (e.g. so a signed-out visitor sees an author's
	// name on a post instead of their raw id).
	optional := func(h http.HandlerFunc) http.Handler {
		return optionalAuth(s.verifier, h)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.Debug)
	mux.HandleFunc("/debug", s.Debug)
	mux.Handle("GET /blogs", optional(s.ListBlogs))
	// A post is addressed by its slug alone, since slugs are unique across every author rather than
	// only within one: "hello-world" names at most one post anywhere, and the second post under
	// that title takes a suffixed slug instead (see entity.NewBlogSlug). The author is reported as
	// a field on the response rather than as part of the address - the uid a post records its owner
	// by is never public, and now nothing has to resolve one to reach a post.
	mux.Handle("GET /blogs/{slug}", optional(s.GetBlog))
	mux.Handle("POST /blogs", authed(s.CreateBlog))
	mux.Handle("PUT /blogs/{slug}", authed(s.UpdateBlog))
	mux.Handle("DELETE /blogs/{slug}", authed(s.DeleteBlog))
	// A profile is addressed by its username, never by the Google `sub` it is keyed by, so the id
	// stays an internal detail rather than a public handle. "me" is the one exception, for a client
	// that holds a credential but does not yet know which name it was given; ServeMux prefers that
	// literal over the wildcard, so it wins without any ordering care here.
	mux.Handle("GET /users/me", authed(s.GetCurrentUser))
	mux.Handle("PUT /users/me", authed(s.PutUser))
	mux.Handle("DELETE /users/me", authed(s.DeleteUser))
	mux.Handle("GET /users/{username}", optional(s.GetUser))

	// CORS wraps the whole mux rather than individual routes: routes are registered under a
	// specific method, so ServeMux would 405 an OPTIONS preflight before a per-route wrapper ran.
	return withCORS(s.cfg.AllowedOrigin, mux)
}
