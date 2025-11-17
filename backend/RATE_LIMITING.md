# Rate Limiting Implementation

## Overview

Implemented comprehensive rate limiting to prevent brute force attacks, denial of service, and API abuse. Uses industry-standard token bucket algorithm with interface abstraction for easy migration to distributed systems.

## Architecture

### Interface-Based Design

```go
type RateLimiter interface {
    Allow(ctx context.Context, key string) bool
    AllowN(ctx context.Context, key string, n int) bool
}
```

**Why interface:**
- Easy to swap implementations (in-memory → Redis)
- Testable with mock implementations
- No code changes needed when scaling

**Current implementation:** `InMemoryRateLimiter`
**Future option:** `RedisRateLimiter` (drop-in replacement)

---

## Rate Limits Applied

### Global Rate Limit
**Applied to:** All endpoints
**Limit:** 100 requests/second per IP
**Burst:** 200 requests
**Purpose:** Prevent general DoS attacks while allowing normal usage

```
User makes 150 requests in 1 second:
- First 200 allowed (burst)
- Next 100/sec allowed
- Additional requests: 429 Rate Limit Exceeded
```

---

### Auth Endpoint Limit
**Applied to:**
- `/auth/github/login`
- `/auth/github/callback`
- `/auth/logout`
- `/auth/cli/authorize`

**Limit:** 10 requests/minute per IP (0.167 req/sec)
**Burst:** 5 requests
**Purpose:** Prevent brute force on authentication flow

```
Attacker tries 100 login attempts:
- First 5 succeed (burst)
- Next 10/minute allowed
- After minute 1: Only 15 attempts made (vs 100 attempted)
- Attack significantly slowed
```

---

### Upload Endpoint Limit
**Applied to:** `/api/v1/sessions/save`
**Limit:** 20 requests/hour per IP (0.0056 req/sec)
**Burst:** 5 requests
**Purpose:** Prevent storage abuse and excessive S3 costs

```
Normal user:
- Uploads 3 sessions/hour → no problem

Abuser:
- Tries to upload 1000 sessions → blocked after 25
- Would cost $$$$ in S3 storage → prevented
```

---

## Token Bucket Algorithm

### How It Works

Imagine a bucket that:
1. Holds tokens (e.g., 200 capacity)
2. Tokens refill at steady rate (e.g., 100/second)
3. Each request costs 1 token
4. No tokens = request denied

```
Time 0s:  Bucket has 200 tokens (burst)
          User makes 150 requests → 50 tokens left

Time 1s:  +100 tokens refilled → 150 tokens
          User makes 100 requests → 50 tokens left

Time 2s:  +100 tokens refilled → 150 tokens
          User makes 200 requests → denied (only 150 available)
```

**Benefits:**
- Allows bursts (better UX than strict limits)
- Smooth rate limiting (not on/off)
- Industry standard (used by AWS, Cloudflare, etc.)

---

## Implementation Details

### File Structure

```
backend/internal/ratelimit/
├── ratelimit.go      # Interface + InMemoryRateLimiter
└── middleware.go     # HTTP middleware + helpers
```

### InMemoryRateLimiter

**Location:** `internal/ratelimit/ratelimit.go`

```go
type InMemoryRateLimiter struct {
    rate  rate.Limit      // Requests per second
    burst int             // Maximum burst size
    limiters sync.Map     // Per-key rate limiters
    lastAccess sync.Map   // Cleanup tracking
}
```

**Features:**
- ✅ Per-IP rate limiting
- ✅ Token bucket algorithm (golang.org/x/time/rate)
- ✅ Automatic cleanup of inactive limiters
- ✅ Thread-safe (sync.Map)
- ✅ Zero dependencies (just stdlib + x/time)

**Cleanup:**
- Runs every 5 minutes
- Removes limiters inactive for >10 minutes
- Prevents memory leaks from one-time IPs

---

### Middleware

**Location:** `internal/ratelimit/middleware.go`

```go
func Middleware(limiter RateLimiter) func(http.Handler) http.Handler
func HandlerFunc(limiter RateLimiter, handler http.HandlerFunc) http.HandlerFunc
func MiddlewareWithKey(limiter RateLimiter, keyFunc func(*http.Request) string) func(http.Handler) http.Handler
```

**IP extraction:**
- Checks `X-Forwarded-For` (for proxies/load balancers)
- Falls back to `X-Real-IP`
- Falls back to `RemoteAddr`
- Handles IPv6 and port stripping

---

### Server Integration

**Location:** `internal/api/server.go`

```go
type Server struct {
    globalLimiter   RateLimiter  // 100 req/sec
    authLimiter     RateLimiter  // 10 req/min
    uploadLimiter   RateLimiter  // 20 req/hour
}

// Applied as middleware
r.Use(ratelimit.Middleware(s.globalLimiter))

// Applied to specific endpoints
r.Get("/auth/github/login",
    ratelimit.HandlerFunc(s.authLimiter, auth.HandleGitHubLogin(s.oauthConfig)))

// Applied to route group
r.Group(func(r chi.Router) {
    r.Use(ratelimit.Middleware(s.uploadLimiter))
    r.Post("/sessions/save", s.handleSaveSession)
})
```

---

## Attack Scenarios Prevented

### Attack 1: Brute Force OAuth

**Before:**
```bash
# Attacker tries 1000 OAuth login attempts
for i in {1..1000}; do
    curl http://localhost:8080/auth/github/login
done

# Result: All 1000 succeed, GitHub gets flooded
```

**After:**
```bash
# Same attack
# First 5 succeed (burst)
# Next 10/minute allowed
# After 1 minute: 15 total vs 1000 attempted
# After 10 minutes: 105 total

# Attack severely rate limited
```

---

### Attack 2: DoS via Uploads

**Before:**
```bash
# Attacker floods upload endpoint
while true; do
    curl -X POST /api/v1/sessions/save \
      -H "Authorization: Bearer $KEY" \
      -d @50mb-file.json
done

# Result: S3 fills up, $10,000 bill
```

**After:**
```bash
# Same attack
# First 5 succeed (burst)
# Next 20/hour allowed
# 25 total in first hour
# 25 files × 50MB = 1.25GB (vs infinite)

# Attack contained
```

---

### Attack 3: Distributed DoS

**Before:**
```bash
# Attacker uses 100 IPs
for ip in $(cat 100-ips.txt); do
    curl --interface $ip http://localhost:8080/health
done

# Result: Server overwhelmed, crashes
```

**After:**
```bash
# Same attack
# Each IP limited to 100 req/sec
# 100 IPs × 100 req/sec = 10,000 req/sec max
# Still high but bounded

# For real DDoS: Add Cloudflare/AWS WAF
```

---

## Testing

### Test 1: Normal Usage (Should Succeed)

```bash
# 5 requests in quick succession
for i in {1..5}; do
    curl http://localhost:8080/health
done

# Expected: All succeed (within burst)
```

### Test 2: Exceed Global Limit (Should Fail)

```bash
# 300 requests in 1 second (exceeds 200 burst)
for i in {1..300}; do
    curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/health &
done
wait

# Expected: First ~200 return 200, rest return 429
```

### Test 3: Auth Rate Limiting (Should Fail)

```bash
# 20 auth requests in 1 minute (exceeds 10/min + 5 burst)
for i in {1..20}; do
    curl -s -o /dev/null -w "%{http_code}\n" \
        http://localhost:8080/auth/github/login
    sleep 3  # 20 requests over 60 seconds
done

# Expected: First 15 succeed, last 5 get 429
```

### Test 4: Upload Rate Limiting (Should Fail)

```bash
# 10 uploads in 1 hour (exceeds 5 burst, but within 20/hour)
for i in {1..10}; do
    curl -X POST http://localhost:8080/api/v1/sessions/save \
        -H "Authorization: Bearer $API_KEY" \
        -d '{"session_id":"test'$i'","files":[]}'
    sleep 360  # 1 hour / 10 = 6 minutes
done

# Expected: All succeed (within limits)
```

### Test 5: Recovery After Rate Limit

```bash
# Trigger rate limit
for i in {1..300}; do
    curl http://localhost:8080/health &
done
wait

# Wait for bucket to refill
sleep 2

# Try again
curl http://localhost:8080/health

# Expected: Succeeds (tokens refilled)
```

---

## Migrating to Redis (Future)

When you need multiple instances, swap in Redis implementation:

### Step 1: Create RedisRateLimiter

```go
package ratelimit

import "github.com/go-redis/redis_rate/v10"

type RedisRateLimiter struct {
    limiter *redis_rate.Limiter
}

func NewRedisRateLimiter(redisClient *redis.Client, rps float64, burst int) *RedisRateLimiter {
    return &RedisRateLimiter{
        limiter: redis_rate.NewLimiter(redisClient),
    }
}

func (r *RedisRateLimiter) Allow(ctx context.Context, key string) bool {
    result, err := r.limiter.Allow(ctx, key, redis_rate.PerSecond(int(rps)))
    if err != nil {
        // Log error, fail open (allow request)
        return true
    }
    return result.Allowed > 0
}

func (r *RedisRateLimiter) AllowN(ctx context.Context, key string, n int) bool {
    // Similar implementation
}
```

### Step 2: Update Server Constructor

```go
// Before
globalLimiter: ratelimit.NewInMemoryRateLimiter(100, 200),

// After
globalLimiter: ratelimit.NewRedisRateLimiter(redisClient, 100, 200),
```

**That's it!** No other code changes needed thanks to interface.

---

## Monitoring

### Metrics to Track

1. **Rate limit hits:**
   ```go
   log.Printf("Rate limit exceeded for IP: %s, path: %s", ip, r.URL.Path)
   ```

2. **Active limiters (memory usage):**
   ```go
   stats := limiter.(*InMemoryRateLimiter).Stats()
   // {"active_limiters": 1234, "rate_per_second": 100, ...}
   ```

3. **Top rate-limited IPs:**
   - Parse logs for "Rate limit exceeded"
   - Alert if single IP repeatedly hit limit (possible attack)

### Recommended Alerts

```bash
# Alert if >100 rate limit events in 5 minutes
# Possible ongoing attack

# Alert if active_limiters >10,000
# Memory leak or massive attack

# Alert if same IP hit limit >1000 times
# Persistent attacker, consider banning
```

---

## Tuning Limits

### If Limits Too Strict

Users reporting 429 errors:

1. **Check actual usage:**
   ```go
   log.Printf("Request from IP %s allowed: %v", ip, allowed)
   ```

2. **Increase limits:**
   ```go
   // Increase burst
   globalLimiter: ratelimit.NewInMemoryRateLimiter(100, 500),

   // Or increase rate
   globalLimiter: ratelimit.NewInMemoryRateLimiter(200, 200),
   ```

3. **Exempt certain IPs (trusted services):**
   ```go
   if ip == "1.2.3.4" {
       next.ServeHTTP(w, r)  // Skip rate limiting
       return
   }
   ```

### If Limits Too Loose

Under attack or high costs:

1. **Tighten limits:**
   ```go
   uploadLimiter: ratelimit.NewInMemoryRateLimiter(0.0028, 3), // 10/hour → 5/hour
   ```

2. **Add per-user limits:**
   ```go
   // Rate limit by user ID instead of IP for uploads
   r.Use(ratelimit.MiddlewareWithKey(s.uploadLimiter,
       ratelimit.UserKeyFunc(auth.UserIDKey)))
   ```

3. **Add endpoint-specific limits:**
   ```go
   // Different limit for different endpoints
   createKeyLimiter := ratelimit.NewInMemoryRateLimiter(1, 5) // 1/sec
   r.Post("/keys", ratelimit.HandlerFunc(createKeyLimiter, HandleCreateAPIKey))
   ```

---

## Best Practices

### ✅ DO

- Use interface for abstraction
- Rate limit by IP for public endpoints
- Rate limit by user ID for authenticated endpoints
- Allow bursts for better UX
- Log rate limit events
- Monitor rate limit metrics
- Set generous limits initially, tighten if needed
- Return 429 status code
- Clean up inactive limiters

### ❌ DON'T

- Hardcode rate limits (use variables)
- Rate limit health checks aggressively
- Forget to handle proxy headers (X-Forwarded-For)
- Set limits too low (frustrates users)
- Ignore rate limit logs (missed attacks)
- Use strict limits without bursts (poor UX)
- Rate limit by session (can be bypassed)

---

## Configuration

### Environment Variables (Future)

Make limits configurable:

```bash
export RATE_LIMIT_GLOBAL=100        # req/sec
export RATE_LIMIT_AUTH=0.167        # req/sec (10/min)
export RATE_LIMIT_UPLOAD=0.0056     # req/sec (20/hour)
```

```go
func getEnvFloat(key string, defaultVal float64) float64 {
    if val := os.Getenv(key); val != "" {
        if f, err := strconv.ParseFloat(val, 64); err == nil {
            return f
        }
    }
    return defaultVal
}

globalRate := getEnvFloat("RATE_LIMIT_GLOBAL", 100)
globalLimiter := ratelimit.NewInMemoryRateLimiter(globalRate, int(globalRate*2))
```

---

## References

- [Token Bucket Algorithm](https://en.wikipedia.org/wiki/Token_bucket)
- [golang.org/x/time/rate Documentation](https://pkg.go.dev/golang.org/x/time/rate)
- [OWASP: Denial of Service](https://owasp.org/www-community/attacks/Denial_of_Service)
- [RFC 6585: HTTP Status Code 429](https://datatracker.ietf.org/doc/html/rfc6585#section-4)
- [Cloudflare: Rate Limiting](https://www.cloudflare.com/learning/bots/what-is-rate-limiting/)

---

## Changelog

### 2025-01-16: Initial Implementation

- Created `RateLimiter` interface for abstraction
- Implemented `InMemoryRateLimiter` with token bucket algorithm
- Added HTTP middleware with IP extraction
- Applied three-tier rate limiting:
  - Global: 100 req/sec (all endpoints)
  - Auth: 10 req/min (OAuth flow)
  - Upload: 20 req/hour (storage protection)
- Added automatic cleanup of inactive limiters
- Prepared for Redis migration with interface design
- Created comprehensive documentation

---

## Future Enhancements

### Per-User Rate Limiting

For authenticated endpoints, rate limit by user ID:

```go
r.Use(ratelimit.MiddlewareWithKey(
    uploadLimiter,
    ratelimit.UserKeyFunc(auth.UserIDKey),
))
```

### Dynamic Rate Limits

Adjust limits based on user tier:

```go
func getRateLimit(userID int64) float64 {
    tier := getUserTier(userID)
    switch tier {
    case "free":
        return 10
    case "pro":
        return 100
    case "enterprise":
        return 1000
    }
}
```

### IP Allowlist/Blocklist

```go
var blockedIPs = map[string]bool{
    "1.2.3.4": true,
}

if blockedIPs[ip] {
    http.Error(w, "Forbidden", 403)
    return
}
```

### Geolocation-Based Limits

```go
country := geolocate(ip)
if country == "XX" {
    // Apply stricter limits to high-risk regions
    limiter = strictLimiter
}
```
