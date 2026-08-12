package auth

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiterConfig is deliberately one shared pair of numbers for every
// guarded endpoint, not per-endpoint tunables — Stage 26A1's scope is
// "add rate limiting," not "give every endpoint its own dial." Per-endpoint
// isolation still holds because the bucket key includes the endpoint name
// (see RateLimiter.Limit), so /auth/login and /auth/register never share a
// counter even though they're configured with the same numbers and may see
// the same client IP.
type RateLimiterConfig struct {
	MaxAttempts int
	Window      time.Duration
}

// RateLimiter is a simple in-process, fixed-window counter keyed by
// (endpoint, client IP) — no Redis, no DB table. This backend runs as a
// single replica (confirmed: no `replicas`/`scale` entry for the `backend`
// service in docker-compose.yml, no reverse-proxy/load-balancer in front of
// it), so in-process state is a correct, not just convenient, choice: every
// request really does land on the one process holding the counters. Should
// this backend ever run more than one replica, this in-process approach
// would under-count (each replica sees only its own share of traffic) and
// would need to move to a shared store — deliberately not attempted now,
// per the roadmap's own "decide during the session" framing for Stage 26.
//
// Bounded by construction: stale buckets are swept periodically (see
// cleanupLoop) rather than left to accumulate forever, so long uptime with
// many distinct source IPs (including scanners) doesn't grow this map
// without limit.
type RateLimiter struct {
	cfg     RateLimiterConfig
	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	count      int
	windowFrom time.Time
}

// cleanupInterval/cleanupIdleAfter govern the background sweep — generous
// multiples of a typical window so a bucket is never evicted while it could
// still matter, but idle entries don't linger indefinitely.
const (
	cleanupInterval  = 10 * time.Minute
	cleanupIdleAfter = 30 * time.Minute
)

func NewRateLimiter(cfg RateLimiterConfig) *RateLimiter {
	rl := &RateLimiter{cfg: cfg, buckets: make(map[string]*bucket)}
	go rl.cleanupLoop()
	return rl
}

func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-cleanupIdleAfter)
		rl.mu.Lock()
		for key, b := range rl.buckets {
			if b.windowFrom.Before(cutoff) {
				delete(rl.buckets, key)
			}
		}
		rl.mu.Unlock()
	}
}

// Limit returns Gin middleware enforcing this limiter for one logical
// endpoint name, baked into the bucket key. Counts every request that
// reaches this middleware — successful or failed — toward the same window;
// this is deliberate (a credential-stuffing script's occasional successful
// guess should throttle exactly like its failures do), and never changes
// what a request that's still under the limit receives: the real
// handler runs completely untouched, with the exact same success/failure
// behavior it always had. Only requests past the threshold are turned away,
// before ever reaching Register/Login.
func (rl *RateLimiter) Limit(endpoint string) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := endpoint + "|" + clientIP(c)
		now := time.Now()

		rl.mu.Lock()
		b, ok := rl.buckets[key]
		if !ok || now.Sub(b.windowFrom) >= rl.cfg.Window {
			b = &bucket{count: 0, windowFrom: now}
			rl.buckets[key] = b
		}
		b.count++
		exceeded := b.count > rl.cfg.MaxAttempts
		retryAfter := rl.cfg.Window - now.Sub(b.windowFrom)
		rl.mu.Unlock()

		if exceeded {
			seconds := int(retryAfter.Seconds())
			if seconds < 1 {
				seconds = 1
			}
			c.Header("Retry-After", strconv.Itoa(seconds))
			respondError(c, http.StatusTooManyRequests, "RATE_LIMITED", "too many attempts, please try again later")
			c.Abort()
			return
		}
		c.Next()
	}
}

// clientIP deliberately does NOT use gin's own c.ClientIP(). This backend
// runs via gin.Default() with no SetTrustedProxies call (see cmd/api/main.go),
// so c.ClientIP() honors an X-Forwarded-For/X-Real-IP header from ANY
// caller — trivially spoofable by an attacker rotating the header value
// per request to hand this exact limiter a fresh "IP" every time and bypass
// it entirely. RemoteAddr is set by Go's net/http server from the actual
// TCP connection, never from client-controlled request data, matching this
// stage's explicit "must not trust spoofable ... fields" requirement.
//
// Trade-off, stated plainly: this becomes wrong the day a real reverse
// proxy sits in front of this backend (every request would then show the
// proxy's own address, bucketing every real client behind it together) —
// there is none today (confirmed above), so that day hasn't come. Revisit
// alongside a real SetTrustedProxies configuration when it does, not before.
func clientIP(c *gin.Context) string {
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		return c.Request.RemoteAddr
	}
	return host
}
