package service

import (
	"fmt"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Every budget below is held in memory by one process, so it is a per-instance budget: a service
// running on N Cloud Run instances admits N times these numbers, and a caller whose requests land
// on different instances is metered separately on each. That is deliberate for now - this service
// runs as a single instance - and it is the one thing to revisit before scaling out, where the
// counters would have to move to a store every instance shares (Firestore, Redis). Nothing outside
// this file knows where a bucket lives, so that change is contained to rateLimiter.
var (
	// requestsPerIP bounds the total volume one client can put through the API, whether or not it
	// is signed in. It sits in front of everything, so it is also what stops an anonymous flood
	// from spending the token verifier's time on credentials it invented.
	requestsPerIP = rateLimit{burst: 60, every: time.Second}
	// requestsPerCaller bounds a single verified account across every route that requires one -
	// which is every write. It is deliberately tighter than the per-IP budget above: an account is
	// harder to come by than an address, so a lower ceiling costs a real user nothing.
	requestsPerCaller = rateLimit{burst: 20, every: 3 * time.Second}
	// assistantTurnsPerCaller bounds the one route that costs real money to serve: a chat turn
	// calls Gemini and can hold a request open for two minutes. The burst covers an author
	// iterating on a paragraph; the refill rate is what an author writing, rather than hammering,
	// will never notice.
	assistantTurnsPerCaller = rateLimit{burst: 5, every: 30 * time.Second}
)

// rateLimitSweepInterval is how often idle buckets are dropped. See rateLimiter.sweep.
const rateLimitSweepInterval = time.Minute

// rateLimit is one budget: burst requests admitted at once, refilled at one per every.
type rateLimit struct {
	burst int
	every time.Duration
}

// tokenBucket is one key's remaining budget, refilled from the clock rather than by a timer, so an
// idle key costs nothing to keep current.
type tokenBucket struct {
	tokens float64
	at     time.Time
}

// refill credits the time since this bucket was last touched, up to a full burst.
func (b *tokenBucket) refill(limit rateLimit, now time.Time) {
	elapsed := now.Sub(b.at)
	if elapsed <= 0 {
		// A clock that did not advance (or went backwards) credits nothing, and must not move the
		// timestamp forward either, or the elapsed time would be lost rather than deferred.
		return
	}
	b.at = now
	b.tokens = min(float64(limit.burst), b.tokens+float64(elapsed)/float64(limit.every))
}

// rateLimiter meters requests per key against one budget, in the memory of this process.
type rateLimiter struct {
	limit rateLimit
	// now is the clock, swappable so a test can exhaust and refill a bucket without sleeping.
	now func() time.Time

	mu      sync.Mutex
	buckets map[string]*tokenBucket
	sweptAt time.Time
}

func newRateLimiter(limit rateLimit) *rateLimiter {
	return &rateLimiter{
		limit:   limit,
		now:     time.Now,
		buckets: make(map[string]*tokenBucket),
	}
}

// allow spends one token for key, reporting how long the caller must wait when there is none left.
// An unknown key starts full, so the first request from anybody is always admitted.
func (l *rateLimiter) allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.sweep(now)

	bucket, ok := l.buckets[key]
	if !ok {
		bucket = &tokenBucket{tokens: float64(l.limit.burst), at: now}
		l.buckets[key] = bucket
	}
	bucket.refill(l.limit, now)

	if bucket.tokens < 1 {
		return false, time.Duration((1 - bucket.tokens) * float64(l.limit.every))
	}
	bucket.tokens--
	return true, 0
}

// sweep drops buckets that have refilled to full. Such a bucket holds no information: a key with a
// full bucket and a key that has never been seen admit exactly the same requests, so forgetting it
// changes nothing a caller can observe. Doing it here, on a bucket that is already being consulted
// under the lock, means the map stays bounded by the number of *active* keys without a goroutine
// or a timer to own - and the interval keeps a busy service from walking the whole map per request.
//
// Callers must hold l.mu.
func (l *rateLimiter) sweep(now time.Time) {
	if now.Sub(l.sweptAt) < rateLimitSweepInterval {
		return
	}
	l.sweptAt = now

	for key, bucket := range l.buckets {
		bucket.refill(l.limit, now)
		if bucket.tokens >= float64(l.limit.burst) {
			delete(l.buckets, key)
		}
	}
}

// rateLimited rejects a request whose key has spent its budget, and passes everything else
// through. The 429 carries the same JSON error shape as every other failure, plus a Retry-After
// header for a client that reads one; the wait is repeated in the message because a browser cannot
// see a header this API does not expose to it, and telling an author "try again in 24s" is the
// whole of what they need to know.
func rateLimited(limiter *rateLimiter, key func(*http.Request) string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if allowed, wait := limiter.allow(key(r)); !allowed {
			// Rounded up, and never below the one second Retry-After can express: a client that
			// took "0" literally would come straight back to another 429.
			seconds := max(1, int(math.Ceil(wait.Seconds())))
			w.Header().Set("Retry-After", strconv.Itoa(seconds))
			writeError(w, http.StatusTooManyRequests,
				fmt.Sprintf("too many requests, try again in %ds", seconds))
			return
		}
		next(w, r)
	}
}

// callerKey meters a request against the account that made it, for a route behind requireAuth -
// where the identity came out of a signed token and so is not something a caller can rotate
// through the way it can rotate through addresses.
func callerKey(r *http.Request) string {
	return uidFromContext(r.Context())
}

// clientIP is the address the request arrived from, for metering callers who have not signed in.
//
// The rightmost X-Forwarded-For entry is used rather than the leftmost, which is the one usually
// wanted for logging but is worthless here: a client can put anything at the front of that header,
// and Cloud Run appends the address it actually saw rather than replacing it. Reading the front
// would let anybody dodge this limiter with a random header per request. The rightmost entry is
// written by the hop nearest this service, which for a Cloud Run service called directly (see
// infrastructure/env/cloud_run.tf - there is no load balancer in front of it) is the real client.
// Adding a proxy in front would move the client one entry to the left and make every request share
// one bucket, which fails safe: over-limiting, never under-limiting.
func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		if comma := strings.LastIndex(forwarded, ","); comma >= 0 {
			forwarded = forwarded[comma+1:]
		}
		if address := strings.TrimSpace(forwarded); address != "" {
			return address
		}
	}

	// The direct peer, which carries a port that has to come off or every connection from one
	// client would be a key of its own.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
