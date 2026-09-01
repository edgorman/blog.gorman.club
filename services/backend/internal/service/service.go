// Package service configures the API server, its routes, and the business logic that spans more
// than one entity.
package service

import (
	"net/http"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
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
	// AssistantAllowlist decides which profiles may use the AI writing assistant. An empty list
	// disables it for everybody, which is what a deployment with no model configured looks like
	// (see cmd/backend).
	AssistantAllowlist entity.AssistantAllowlist
}

// Service owns the API's dependencies and serves its routes.
type Service struct {
	cfg       Config
	blogs     repository.BlogRepository
	users     repository.UserRepository
	chats     repository.ChatRepository
	verifier  repository.TokenVerifier
	assistant repository.Assistant
	// The rate limiters live on the Service rather than being built in Handler(), so a budget is
	// spent by the service that served the request rather than by the handler tree - two calls to
	// Handler() must not hand a caller two budgets. See ratelimit.go for what each one bounds.
	ipLimiter        *rateLimiter
	callerLimiter    *rateLimiter
	assistantLimiter *rateLimiter
}

func New(
	cfg Config,
	blogs repository.BlogRepository,
	users repository.UserRepository,
	chats repository.ChatRepository,
	verifier repository.TokenVerifier,
	assistant repository.Assistant,
) *Service {
	return &Service{
		cfg:              cfg,
		blogs:            blogs,
		users:            users,
		chats:            chats,
		verifier:         verifier,
		assistant:        assistant,
		ipLimiter:        newRateLimiter(requestsPerIP),
		callerLimiter:    newRateLimiter(requestsPerCaller),
		assistantLimiter: newRateLimiter(assistantTurnsPerCaller),
	}
}

// Handler returns the fully-wired API, ready to serve.
func (s *Service) Handler() http.Handler {
	// Every write requires a verified caller, since it always resolves to a specific owner. The
	// per-account budget sits inside the verification rather than in front of it, because the
	// account it meters is not known until the credential has been checked; an unverified flood is
	// already bounded by the per-IP budget below, which is what keeps it from reaching the
	// verifier in the first place.
	authed := func(h http.HandlerFunc) http.Handler {
		return requireAuth(s.verifier, rateLimited(s.callerLimiter, callerKey, h))
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
	// The assistant conversation hangs off the post it is about rather than living at a collection
	// of its own, because that is exactly what it is: a chat has no identity apart from its post,
	// and no route here could name one that a /blogs/{slug} route would not have resolved first.
	// Every one of them requires the caller to own the post and to be on the assistant allowlist.
	mux.Handle("GET /blogs/{slug}/chat", authed(s.GetChat))
	// A chat turn is metered a second time, against a much smaller budget: it is the only request
	// here that calls a paid model and can hold a connection open for two minutes, so the general
	// per-account allowance is far too loose to be the only thing standing in front of it.
	mux.Handle("POST /blogs/{slug}/chat", authed(rateLimited(s.assistantLimiter, callerKey, s.SendChatMessage)))
	mux.Handle("DELETE /blogs/{slug}/chat", authed(s.DeleteChat))

	// The per-IP budget wraps the whole mux, so it applies to the routes that admit anonymous
	// callers too - the ones with no account to meter - and to a request for a path that does not
	// exist, which a per-route wrapper would never see.
	//
	// CORS wraps that in turn rather than individual routes: routes are registered under a
	// specific method, so ServeMux would 405 an OPTIONS preflight before a per-route wrapper ran.
	// Being outermost also means a preflight is answered without spending a token - a browser
	// sends one per request it makes, and charging for both would halve every budget here - and
	// that a 429 still carries the CORS headers, without which the browser would show the frontend
	// a network error instead of the reason it was refused.
	return withCORS(s.cfg.AllowedOrigin, rateLimited(s.ipLimiter, clientIP, mux.ServeHTTP))
}
