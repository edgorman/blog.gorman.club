// Package service configures the API server, its routes, and the business logic that spans more
// than one entity.
package service

import (
	"log/slog"
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/api"
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
	// The rate limiters live on the Service rather than being built in Handler(), so a budget is
	// spent by the service that served the request rather than by the handler tree - two calls to
	// Handler() must not hand a caller two budgets. See ratelimit.go for what each one bounds.
	ipLimiter        *rateLimiter
	callerLimiter    *rateLimiter
	assistantLimiter *rateLimiter
	searchLimiter    *rateLimiter
	// authModes is how each route treats a credential, keyed by its mux pattern ("GET /blogs/{slug}").
	authModes map[string]authMode
}

// authMode is how a route treats a credential. It is read from the route's `security` in
// packages/api/openapi.yaml, so the contract is the one place that says which routes need a caller.
type authMode int

const (
	// authNone routes never look at a credential: /health and /debug.
	authNone authMode = iota
	// authOptional routes verify a credential if one is sent, for reads that admit anonymous callers
	// but still authorize a signed-in one. Their `security` lists an empty requirement ({}).
	authOptional
	// authRequired routes need a verified caller, and meter it per account.
	authRequired
)

// routeAuthModes reads every operation's authMode out of the embedded OpenAPI spec.
func routeAuthModes() (map[string]authMode, error) {
	spec, err := api.GetSwagger()
	if err != nil {
		return nil, err
	}

	modes := make(map[string]authMode)
	for path, item := range spec.Paths.Map() {
		for method, operation := range item.Operations() {
			modes[method+" "+path] = authModeOf(operation.Security)
		}
	}
	return modes, nil
}

func authModeOf(security *openapi3.SecurityRequirements) authMode {
	if security == nil || len(*security) == 0 {
		return authNone
	}
	for _, requirement := range *security {
		if len(requirement) == 0 {
			return authOptional
		}
	}
	return authRequired
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
) *Service {
	// The spec is generated into the binary, so failing to read it is a broken build, not a
	// condition to handle at runtime.
	modes, err := routeAuthModes()
	if err != nil {
		panic("service: reading the embedded OpenAPI spec: " + err.Error())
	}

	return &Service{
		authModes:        modes,
		cfg:              cfg,
		blogs:            blogs,
		users:            users,
		chats:            chats,
		comments:         comments,
		reactions:        reactions,
		embeddings:       embeddings,
		verifier:         verifier,
		assistant:        assistant,
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

// Handler returns the fully-wired API, ready to serve. The routes come from the generated
// api.HandlerWithOptions, so a route exists exactly when packages/api/openapi.yaml declares it and
// this Service implements the operation (the compiler checks api.ServerInterface).
func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	api.HandlerWithOptions(s, api.StdHTTPServerOptions{BaseRouter: mux, ErrorHandlerFunc: badRequest})

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
	return withCORS(s.cfg.AllowedOrigin, rateLimited(s.ipLimiter, clientIP, s.authenticate(mux)))
}

// authenticate applies each route's authMode before the mux dispatches to it, so a bad credential
// is refused before the route's parameters are even read.
//
// Every write requires a verified caller, since it always resolves to a specific owner. The
// per-account budget sits inside the verification rather than in front of it, because the account
// it meters is not known until the credential has been checked; an unverified flood is already
// bounded by the per-IP budget, which is what keeps it from reaching the verifier in the first
// place. Reads that admit anonymous callers are authorized downstream: entity.Blog.CanBeReadBy
// treats the zero Caller as seeing only public posts, and a profile has nothing to hide.
//
// The two routes that call a paid model are metered a second time inside their handlers (see
// ListBlogs and SendChatMessage), since that needs the verified caller this has just resolved.
func (s *Service) authenticate(mux *http.ServeMux) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// The pattern is empty for a path or method the mux does not serve, which is authNone: the
		// mux answers it with its own 404 or 405.
		_, pattern := mux.Handler(r)
		switch s.authModes[pattern] {
		case authRequired:
			requireAuth(s.verifier, rateLimited(s.callerLimiter, callerKey, mux.ServeHTTP))(w, r)
		case authOptional:
			optionalAuth(s.verifier, mux.ServeHTTP)(w, r)
		default:
			mux.ServeHTTP(w, r)
		}
	}
}
