package service

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/api"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

// The contract must be a valid OpenAPI document, which is what the generators and the typed client
// assume: this is the spec's linter.
func TestSpec_IsValid(t *testing.T) {
	spec, err := api.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	if err := spec.Validate(t.Context()); err != nil {
		t.Errorf("packages/api/openapi.yaml is not a valid OpenAPI document: %v", err)
	}
}

// Which routes need a caller is read from each operation's `security` in the spec, so this pins the
// reading against the routes the API actually has: a write that lost its `security` would become
// reachable anonymously.
func TestRouteAuthModes(t *testing.T) {
	modes, err := routeAuthModes()
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]authMode{
		"GET /health":                                          authNone,
		"GET /debug":                                           authNone,
		"GET /blogs":                                           authOptional,
		"POST /blogs":                                          authRequired,
		"GET /blogs/{slug}":                                    authOptional,
		"PUT /blogs/{slug}":                                    authRequired,
		"DELETE /blogs/{slug}":                                 authRequired,
		"GET /blogs/{slug}/related":                            authOptional,
		"GET /blogs/{slug}/chat":                               authRequired,
		"POST /blogs/{slug}/chat":                              authRequired,
		"DELETE /blogs/{slug}/chat":                            authRequired,
		"GET /blogs/{slug}/comments":                           authOptional,
		"POST /blogs/{slug}/comments":                          authRequired,
		"DELETE /blogs/{slug}/comments/{id}":                   authRequired,
		"PUT /blogs/{slug}/comments/{id}/moderation":           authRequired,
		"GET /blogs/{slug}/reactions":                          authOptional,
		"PUT /blogs/{slug}/reactions/{emoji}":                  authRequired,
		"DELETE /blogs/{slug}/reactions/{emoji}":               authRequired,
		"PUT /blogs/{slug}/comments/{id}/reactions/{emoji}":    authRequired,
		"DELETE /blogs/{slug}/comments/{id}/reactions/{emoji}": authRequired,
		"GET /users/me":                                        authRequired,
		"PUT /users/me":                                        authRequired,
		"DELETE /users/me":                                     authRequired,
		"GET /users/{username}":                                authOptional,
	}
	for route, mode := range want {
		if got, ok := modes[route]; !ok || got != mode {
			t.Errorf("%s = %v (declared: %v), want %v", route, got, ok, mode)
		}
	}
	if len(modes) != len(want) {
		t.Errorf("the spec declares %d routes, this test knows %d: add the new ones", len(modes), len(want))
	}
}

// The auth modes read from the spec must actually be applied by the handler: a write refused
// without a credential, a read admitting an anonymous caller but refusing a bad credential, and
// the debug routes never looking at one.
func TestHandler_AppliesEachRoutesAuthMode(t *testing.T) {
	status := func(verifier repository.TokenVerifier, method, path string, signedIn bool) int {
		s := newTestService(newFakeBlogRepository(), newFakeUserRepository())
		s.verifier = verifier
		req := httptest.NewRequest(method, path, nil)
		if signedIn {
			req.Header.Set(authorizationHeader, bearerPrefix+"token")
			req.Header.Set(authorizationProviderHeader, string(providerGoogle))
		}
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, req)
		return rec.Result().StatusCode
	}
	good, bad := fakeVerifier{uid: "caller"}, fakeVerifier{err: errors.New("bad token")}

	tests := []struct {
		name     string
		verifier repository.TokenVerifier
		method   string
		path     string
		signedIn bool
		want     int
	}{
		{"required, anonymous", good, http.MethodPost, "/blogs", false, http.StatusUnauthorized},
		{"required, bad credential", bad, http.MethodGet, "/users/me", true, http.StatusUnauthorized},
		{"optional, anonymous", good, http.MethodGet, "/blogs", false, http.StatusOK},
		{"optional, good credential", good, http.MethodGet, "/blogs", true, http.StatusOK},
		{"optional, bad credential", bad, http.MethodGet, "/blogs", true, http.StatusUnauthorized},
		{"none, bad credential", bad, http.MethodGet, "/health", true, http.StatusOK},
		{"unknown path", good, http.MethodGet, "/nope", false, http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := status(tt.verifier, tt.method, tt.path, tt.signedIn); got != tt.want {
				t.Errorf("%s %s = %d, want %d", tt.method, tt.path, got, tt.want)
			}
		})
	}
}
