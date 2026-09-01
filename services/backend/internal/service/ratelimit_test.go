package service

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
)

// newTestLimiter builds a limiter on a clock the test moves by hand, so a budget can be exhausted
// and refilled without a sleep anywhere.
func newTestLimiter(limit rateLimit, clock *time.Time) *rateLimiter {
	limiter := newRateLimiter(limit)
	limiter.now = func() time.Time { return *clock }
	return limiter
}

// freezeLimiters stops the clock behind every one of a service's budgets, so a test asserting on a
// burst cannot be handed an extra token by the wall clock advancing mid-loop.
func freezeLimiters(s *Service) {
	at := time.Now()
	for _, limiter := range []*rateLimiter{s.ipLimiter, s.callerLimiter, s.assistantLimiter} {
		limiter.now = func() time.Time { return at }
	}
}

func TestRateLimiter_AdmitsBurstThenRefuses(t *testing.T) {
	now := time.Now()
	limiter := newTestLimiter(rateLimit{burst: 3, every: time.Second}, &now)

	for i := range 3 {
		if allowed, _ := limiter.allow("caller"); !allowed {
			t.Fatalf("request %d refused, want the whole burst admitted", i+1)
		}
	}

	allowed, wait := limiter.allow("caller")
	if allowed {
		t.Fatal("request past the burst was admitted")
	}
	if wait != time.Second {
		t.Errorf("wait = %v, want %v (one refill interval)", wait, time.Second)
	}
}

func TestRateLimiter_RefillsOverTime(t *testing.T) {
	now := time.Now()
	limiter := newTestLimiter(rateLimit{burst: 1, every: 10 * time.Second}, &now)

	limiter.allow("caller")
	if allowed, _ := limiter.allow("caller"); allowed {
		t.Fatal("second request admitted from a one-token budget")
	}

	// Part of the way through the wait, the reported wait is what is left of it rather than a
	// whole interval again.
	now = now.Add(4 * time.Second)
	allowed, wait := limiter.allow("caller")
	if allowed {
		t.Fatal("request admitted before a token had refilled")
	}
	if wait != 6*time.Second {
		t.Errorf("wait = %v, want 6s (the remainder of the interval)", wait)
	}

	now = now.Add(6 * time.Second)
	if allowed, _ := limiter.allow("caller"); !allowed {
		t.Error("request refused after a token had refilled")
	}
}

// A bucket never refills past its burst, so an account that was idle for a week is admitted a
// burst and no more.
func TestRateLimiter_RefillIsCappedAtBurst(t *testing.T) {
	now := time.Now()
	limiter := newTestLimiter(rateLimit{burst: 2, every: time.Second}, &now)

	limiter.allow("caller")
	now = now.Add(time.Hour)

	for i := range 2 {
		if allowed, _ := limiter.allow("caller"); !allowed {
			t.Fatalf("request %d refused, want a full burst after a long idle", i+1)
		}
	}
	if allowed, _ := limiter.allow("caller"); allowed {
		t.Error("a long idle credited more than one burst")
	}
}

func TestRateLimiter_KeysAreMeteredSeparately(t *testing.T) {
	now := time.Now()
	limiter := newTestLimiter(rateLimit{burst: 1, every: time.Second}, &now)

	limiter.allow("one")
	if allowed, _ := limiter.allow("one"); allowed {
		t.Fatal("first key was not exhausted")
	}
	if allowed, _ := limiter.allow("two"); !allowed {
		t.Error("second key was refused a budget of its own")
	}
}

// A bucket that has refilled to full is indistinguishable from one that never existed, so the
// sweep drops it - which is what keeps the map bounded by active keys rather than by every key
// ever seen.
func TestRateLimiter_SweepDropsFullBuckets(t *testing.T) {
	now := time.Now()
	limiter := newTestLimiter(rateLimit{burst: 2, every: time.Second}, &now)

	limiter.allow("gone-quiet")
	now = now.Add(rateLimitSweepInterval)
	limiter.allow("still-here")

	if _, kept := limiter.buckets["gone-quiet"]; kept {
		t.Error("a fully refilled bucket was kept")
	}
	if _, kept := limiter.buckets["still-here"]; !kept {
		t.Error("the bucket being spent was dropped")
	}
}

func TestRateLimiter_SweepKeepsSpentBuckets(t *testing.T) {
	now := time.Now()
	limiter := newTestLimiter(rateLimit{burst: 2, every: time.Hour}, &now)

	limiter.allow("busy")
	limiter.allow("busy")
	now = now.Add(rateLimitSweepInterval)
	limiter.allow("other")

	if _, kept := limiter.buckets["busy"]; !kept {
		t.Fatal("a bucket that had not refilled was dropped, handing its key a fresh burst")
	}
	if allowed, _ := limiter.allow("busy"); allowed {
		t.Error("an exhausted key was admitted after a sweep")
	}
}

func TestClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		forwarded  string
		want       string
	}{
		{
			name:       "falls back to the direct peer without its port",
			remoteAddr: "203.0.113.7:54321",
			want:       "203.0.113.7",
		},
		{
			name:       "a peer address with no port is used as-is",
			remoteAddr: "203.0.113.7",
			want:       "203.0.113.7",
		},
		{
			name:       "a single forwarded entry is the client",
			remoteAddr: "10.0.0.1:1234",
			forwarded:  "203.0.113.7",
			want:       "203.0.113.7",
		},
		{
			// The leftmost entries are whatever the client wrote; only the rightmost was added by
			// the hop in front of this service, so a spoofed header cannot dodge the limiter.
			name:       "a spoofed prefix is ignored in favour of the rightmost entry",
			remoteAddr: "10.0.0.1:1234",
			forwarded:  "1.1.1.1, 2.2.2.2, 203.0.113.7",
			want:       "203.0.113.7",
		},
		{
			name:       "a blank rightmost entry falls back to the peer",
			remoteAddr: "203.0.113.7:54321",
			forwarded:  "1.1.1.1,   ",
			want:       "203.0.113.7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/debug", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.forwarded != "" {
				req.Header.Set("X-Forwarded-For", tt.forwarded)
			}

			if got := clientIP(req); got != tt.want {
				t.Errorf("clientIP = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRateLimited_RefusesWithRetryAfter(t *testing.T) {
	now := time.Now()
	limiter := newTestLimiter(rateLimit{burst: 1, every: 30 * time.Second}, &now)

	calls := 0
	handler := rateLimited(limiter, func(*http.Request) string { return "key" }, func(http.ResponseWriter, *http.Request) {
		calls++
	})

	handler(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/debug", nil))

	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/debug", nil))

	if calls != 1 {
		t.Errorf("handler ran %d times, want 1 - a refused request must not reach it", calls)
	}
	if rec.Result().StatusCode != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", rec.Result().StatusCode, http.StatusTooManyRequests)
	}
	if got := rec.Result().Header.Get("Retry-After"); got != "30" {
		t.Errorf("Retry-After = %q, want %q", got, "30")
	}
	decodeAPIError(t, rec)
}

// Retry-After is expressed in whole seconds and cannot say "less than one", so a sub-second wait
// is rounded up rather than reported as a zero the client would come straight back from.
func TestRateLimited_RetryAfterIsNeverZero(t *testing.T) {
	now := time.Now()
	limiter := newTestLimiter(rateLimit{burst: 1, every: 100 * time.Millisecond}, &now)

	handler := rateLimited(limiter, func(*http.Request) string { return "key" }, func(http.ResponseWriter, *http.Request) {})
	handler(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/debug", nil))

	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/debug", nil))

	seconds, err := strconv.Atoi(rec.Result().Header.Get("Retry-After"))
	if err != nil {
		t.Fatalf("Retry-After is not an integer: %v", err)
	}
	if seconds < 1 {
		t.Errorf("Retry-After = %d, want at least 1", seconds)
	}
}

// The per-IP budget covers the routes that admit anonymous callers, which have no account to meter.
func TestHandler_LimitsAnonymousRequestsPerIP(t *testing.T) {
	s := newTestService(nil, nil)
	freezeLimiters(s)
	handler := s.Handler()

	request := func(address string) int {
		req := httptest.NewRequest(http.MethodGet, "/debug", nil)
		req.RemoteAddr = address + ":1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Result().StatusCode
	}

	for i := range requestsPerIP.burst {
		if status := request("203.0.113.7"); status != http.StatusOK {
			t.Fatalf("request %d: status = %d, want %d", i+1, status, http.StatusOK)
		}
	}
	if status := request("203.0.113.7"); status != http.StatusTooManyRequests {
		t.Errorf("status past the burst = %d, want %d", status, http.StatusTooManyRequests)
	}
	if status := request("198.51.100.9"); status != http.StatusOK {
		t.Errorf("status for a second address = %d, want %d - one client must not exhaust another", status, http.StatusOK)
	}
}

// A preflight is answered above the limiter, so a browser's OPTIONS never spends the budget of the
// request it precedes.
func TestHandler_PreflightIsNotRateLimited(t *testing.T) {
	s := New(Config{AllowedOrigin: testOrigin}, newFakeBlogRepository(), newFakeUserRepository(),
		newFakeChatRepository(), newFakeCommentRepository(), fakeVerifier{uid: "caller"}, &fakeAssistant{})
	freezeLimiters(s)
	handler := s.Handler()

	for range requestsPerIP.burst + 10 {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodOptions, "/blogs", nil))
		if rec.Result().StatusCode != http.StatusNoContent {
			t.Fatalf("preflight status = %d, want %d", rec.Result().StatusCode, http.StatusNoContent)
		}
	}
}

// An authenticated route is metered per account, so a caller cannot buy itself more budget by
// changing address.
func TestHandler_LimitsAuthenticatedRequestsPerCaller(t *testing.T) {
	users := newFakeUserRepository()
	users.seed(entity.User{ID: "caller", Username: "calm-smiling-kestrel"})
	s := newTestService(nil, users)
	freezeLimiters(s)
	handler := s.Handler()

	request := func(address string) int {
		req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
		req.RemoteAddr = address + ":1234"
		req.Header.Set(authorizationHeader, bearerPrefix+"token")
		req.Header.Set(authorizationProviderHeader, string(providerGoogle))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Result().StatusCode
	}

	for i := range requestsPerCaller.burst {
		if status := request(fmt.Sprintf("203.0.113.%d", i+1)); status != http.StatusOK {
			t.Fatalf("request %d: status = %d, want %d", i+1, status, http.StatusOK)
		}
	}
	if status := request("198.51.100.9"); status != http.StatusTooManyRequests {
		t.Errorf("status past the burst = %d, want %d (a new address must not reset an account's budget)",
			status, http.StatusTooManyRequests)
	}
}

// The assistant's own budget is far smaller than the per-account one it sits inside, so a chat turn
// is refused long before an ordinary write would be.
func TestHandler_LimitsAssistantTurnsPerCaller(t *testing.T) {
	blogs := newFakeBlogRepository()
	blogs.seed(entity.Blog{
		Slug:       chatSlug,
		OwnerID:    chatOwner,
		Title:      "Hello",
		Content:    "the cat sat",
		Visibility: entity.VisibilityPublic,
	})
	users := newFakeUserRepository()
	users.seed(entity.User{ID: chatOwner, Username: "calm-smiling-kestrel"})

	s := New(
		Config{AssistantAllowlist: entity.NewAssistantAllowlist([]string{chatEmail})},
		blogs, users, newFakeChatRepository(), newFakeCommentRepository(),
		fakeVerifier{uid: chatOwner, email: chatEmail, emailVerified: true},
		&fakeAssistant{},
	)
	freezeLimiters(s)
	handler := s.Handler()

	send := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/blogs/"+chatSlug+"/chat",
			bytes.NewReader([]byte(`{"message":"tighten this"}`)))
		req.Header.Set(authorizationHeader, bearerPrefix+"token")
		req.Header.Set(authorizationProviderHeader, string(providerGoogle))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	if assistantTurnsPerCaller.burst >= requestsPerCaller.burst {
		t.Fatalf("the assistant budget (%d) must be tighter than the per-account one (%d), or it does nothing",
			assistantTurnsPerCaller.burst, requestsPerCaller.burst)
	}
	for i := range assistantTurnsPerCaller.burst {
		if status := send().Result().StatusCode; status != http.StatusOK {
			t.Fatalf("turn %d: status = %d, want %d", i+1, status, http.StatusOK)
		}
	}

	rec := send()
	if rec.Result().StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status past the burst = %d, want %d", rec.Result().StatusCode, http.StatusTooManyRequests)
	}
	decodeAPIError(t, rec)
}
