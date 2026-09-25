// Package service configures the API server, its routes, and the business logic that spans more
// than one entity.
package service

import (
	"log/slog"
	"net/http"
	"sync"

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
	// AssistantEntitlement decides which accounts may use the AI writing assistant. Its zero value
	// is entitled to nobody, which is what a deployment with no model configured looks like (see
	// cmd/backend).
	AssistantEntitlement entity.AssistantEntitlement
	// Logger is where the service writes what an operator reads - failures it answers the caller
	// about only vaguely, and the one line each assistant turn records (see logAssistantTurn). Nil
	// means slog.Default(), which cmd/backend points at Cloud Logging's JSON shape.
	Logger *slog.Logger
}

// Service owns the API's dependencies and serves its routes.
type Service struct {
	cfg       Config
	blogs     repository.BlogRepository
	users     repository.UserRepository
	chats     repository.ChatRepository
	comments  repository.CommentRepository
	reactions repository.ReactionRepository
	// embeddings is only read: the worker writes it (see internal/worker).
	embeddings repository.EmbeddingRepository
	verifier   repository.TokenVerifier
	assistant  repository.Assistant
	payments   repository.Payments
	// webhookMu serializes the billing webhook's fetch-and-write (see StripeWebhook).
	webhookMu sync.Mutex
	// The rate limiters live on the Service rather than being built in Handler(), so a budget is
	// spent by the service that served the request rather than by the handler tree - two calls to
	// Handler() must not hand a caller two budgets. See ratelimit.go for what each one bounds.
	ipLimiter        *rateLimiter
	callerLimiter    *rateLimiter
	assistantLimiter *rateLimiter
	searchLimiter    *rateLimiter
}

func New(
	cfg Config,
	blogs repository.BlogRepository,
	users repository.UserRepository,
	chats repository.ChatRepository,
	comments repository.CommentRepository,
	reactions repository.ReactionRepository,
	embeddings repository.EmbeddingRepository,
	verifier repository.TokenVerifier,
	assistant repository.Assistant,
	payments repository.Payments,
) *Service {
	return &Service{
		cfg:              cfg,
		blogs:            blogs,
		users:            users,
		chats:            chats,
		comments:         comments,
		reactions:        reactions,
		embeddings:       embeddings,
		verifier:         verifier,
		assistant:        assistant,
		payments:         payments,
		ipLimiter:        newRateLimiter(requestsPerIP),
		callerLimiter:    newRateLimiter(requestsPerCaller),
		assistantLimiter: newRateLimiter(assistantTurnsPerCaller),
		searchLimiter:    newRateLimiter(searchesPerClient),
	}
}

// logger is the configured Logger, or the process default when none was given.
func (s *Service) logger() *slog.Logger {
	if s.cfg.Logger != nil {
		return s.cfg.Logger
	}
	return slog.Default()
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
	// A search calls a paid embedding model (see internal/repository/search), so it is metered on
	// its own budget as well, by account or by address for a caller who is not signed in.
	mux.Handle("GET /blogs", optional(searchLimited(s.searchLimiter, s.ListBlogs)))
	// A post is addressed by its slug alone, since slugs are unique across every author rather than
	// only within one: "hello-world" names at most one post anywhere, and the second post under
	// that title takes a suffixed slug instead (see entity.NewBlogSlug). The author is reported as
	// a field on the response rather than as part of the address - the uid a post records its owner
	// by is never public, and now nothing has to resolve one to reach a post.
	mux.Handle("GET /blogs/{slug}", optional(s.GetBlog))
	// Related posts are read like the post they hang under: anonymous callers get the public ones.
	mux.Handle("GET /blogs/{slug}/related", optional(s.RelatedBlogs))
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
	// Every one of them requires the caller to own the post and to hold the assistant entitlement.
	mux.Handle("GET /blogs/{slug}/chat", authed(s.GetChat))
	// A chat turn is metered a second time, against a much smaller budget: it is the only request
	// here that calls a paid model and can hold a connection open for two minutes, so the general
	// per-account allowance is far too loose to be the only thing standing in front of it.
	mux.Handle("POST /blogs/{slug}/chat", authed(rateLimited(s.assistantLimiter, callerKey, s.SendChatMessage)))
	mux.Handle("DELETE /blogs/{slug}/chat", authed(s.DeleteChat))
	// Comments hang off their post for the same reason the chat above does - a comment has no
	// identity apart from the post it replies to - but they are the readers' half rather than the
	// author's, so the rules are the post's own: whoever may read a post may read and write its
	// comments, and a signed-out reader sees a public thread without being able to add to it.
	// Deleting one is neither, being the comment's own rule (see entity.Comment.Permission).
	mux.Handle("GET /blogs/{slug}/comments", optional(s.ListComments))
	mux.Handle("POST /blogs/{slug}/comments", authed(s.CreateComment))
	mux.Handle("DELETE /blogs/{slug}/comments/{id}", authed(s.DeleteComment))
	// The post's owner overrules the classifier here; see ApproveComment.
	mux.Handle("PUT /blogs/{slug}/comments/{id}/moderation", authed(s.ApproveComment))
	// Reactions to a post and to its comments are read together, because they are stored together
	// and a reader opening a post wants the whole page's worth: one route answers what would
	// otherwise be a field on the post plus a field on every comment.
	//
	// Writing one is addressed rather than toggled - PUT puts it there, DELETE takes it back - so
	// a retried click or a stale page lands where it was aiming instead of undoing itself. The
	// emoji is the last segment of the address because it is what identifies the reaction, in the
	// same way the comment id identifies a comment.
	mux.Handle("GET /blogs/{slug}/reactions", optional(s.GetReactions))
	mux.Handle("PUT /blogs/{slug}/reactions/{emoji}", authed(s.PutReaction))
	mux.Handle("DELETE /blogs/{slug}/reactions/{emoji}", authed(s.DeleteReaction))
	mux.Handle("PUT /blogs/{slug}/comments/{id}/reactions/{emoji}", authed(s.PutReaction))
	mux.Handle("DELETE /blogs/{slug}/comments/{id}/reactions/{emoji}", authed(s.DeleteReaction))
	// Billing exists only where it is configured, so a deployment without Stripe keys answers 404
	// here exactly as it would for any path it does not serve. Checkout and the portal act on the
	// caller alone; the webhook has no caller and is authenticated by its signature instead (see
	// StripeWebhook), which is why it is the one write not wrapped in authed.
	if s.billingEnabled() {
		mux.Handle("POST /billing/checkout", authed(s.CreateCheckout))
		mux.Handle("POST /billing/portal", authed(s.CreatePortal))
		mux.HandleFunc("POST /billing/webhook", s.StripeWebhook)
	}

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
