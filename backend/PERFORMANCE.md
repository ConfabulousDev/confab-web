# Performance Optimization Guide

**Last Updated:** 2025-01-21

Complete documentation for performance features in the Confab backend, including rate limiting and response compression.

## Table of Contents

1. [Overview](#overview)
2. [Rate Limiting](#rate-limiting)
3. [Response Compression](#response-compression)
4. [Database Optimization](#database-optimization)
5. [Monitoring](#monitoring)

---

## Overview

### Performance Goals

- **Throughput:** Handle 100 requests/second per server
- **Latency:** P99 response time < 200ms
- **Bandwidth:** 70-90% reduction via compression
- **Availability:** 99.9% uptime
- **Scalability:** Horizontal scaling ready

### Current Optimizations

- ✅ In-memory rate limiting (token bucket algorithm)
- ✅ Dual-encoding compression (Brotli + gzip)
- ✅ Database connection pooling
- ✅ Response size limits
- ✅ Request body size limits
- ⏳ TODO: Redis-based rate limiting for multi-server
- ⏳ TODO: Database query optimization
- ⏳ TODO: CDN integration for static assets

---

## Rate Limiting

### Architecture

**Implementation:** `internal/ratelimit/`

**Algorithm:** Token Bucket (golang.org/x/time/rate)

**Storage:** In-memory (per-process)

```
┌─────────────────────────────────────────┐
│ Request arrives                          │
└──────────────┬───────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│ Extract client key (IP or User ID)      │
└──────────────┬───────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│ Get or create rate limiter for key      │
└──────────────┬───────────────────────────┘
               │
               ▼
       ┌───────────────┐
       │ Allow()?      │
       └───┬───────┬───┘
    YES │       │ NO
        ▼       ▼
    ┌──────┐ ┌────────────────┐
    │ Pass │ │ 429 Too Many   │
    │      │ │ Requests       │
    └──────┘ └────────────────┘
```

### Rate Limit Tiers

#### 1. Global Rate Limiter

**Scope:** All endpoints
**Key:** Client IP address
**Rate:** 100 requests/second
**Burst:** 200

**Purpose:** Prevent DoS attacks

**Example:**
```bash
# Client can burst up to 200 requests immediately
# Then sustained rate of 100 req/sec

# 1000 requests in 1 second
# First 200: ✅ Allowed (burst)
# Next 100: ✅ Allowed (rate)
# Remaining 700: ❌ Rejected (429)
```

#### 2. Auth Rate Limiter

**Scope:** OAuth endpoints
**Key:** Client IP address
**Rate:** 10 requests/minute (0.167/sec)
**Burst:** 5

**Endpoints:**
- `GET /auth/github/login`
- `GET /auth/github/callback`
- `GET /auth/logout`
- `GET /auth/cli/authorize`

**Purpose:** Prevent brute force on authentication

**Example:**
```bash
# Normal user: 1 login attempt every 6 seconds ✅
# Attacker: 100 login attempts in 10 seconds ❌

# First 5: ✅ Allowed (burst)
# Next 1 per 6 seconds: ✅ Allowed (rate = 10/min)
# Any faster: ❌ Rejected (429)
```

#### 3. Upload Rate Limiter

**Scope:** Session uploads
**Key:** User ID (not IP!)
**Rate:** 1000 requests/hour (0.278/sec)
**Burst:** 200

**Endpoints:**
- `POST /api/v1/sessions/save`

**Purpose:** Prevent storage abuse while allowing backfill

**Why User ID?**
- User backfilling 500 sessions needs high burst
- IP-based limiting would block legitimate backfill
- User-based allows burst but prevents abuse per account

**Example:**
```bash
# Backfill scenario (legitimate)
confab cloud backfill ~/.claude/sessions/
# 500 sessions uploaded
# First 200: ✅ Immediate (burst)
# Remaining 300: ✅ At 0.278/sec (~18 minutes)

# Abuse scenario (malicious bot)
# Same user tries to upload 10,000 sessions
# First 200: ✅ Immediate (burst)
# Next 1000/hour: ✅ Allowed by rate
# Beyond 1000/hour: ❌ Rejected (429)
```

#### 4. Validation Rate Limiter

**Scope:** API key validation
**Key:** Client IP address
**Rate:** 30 requests/minute (0.5/sec)
**Burst:** 10

**Endpoints:**
- `GET /api/v1/auth/validate`

**Purpose:** Prevent API key brute forcing

**Example:**
```bash
# CLI checking if key is valid on startup: ✅ Allowed
# Attacker trying 1000 API keys: ❌ Blocked after 10+30/min
```

### Token Bucket Algorithm

**How it works:**

1. Each client has a bucket with tokens
2. Bucket capacity = burst size
3. Tokens refill at configured rate
4. Each request consumes 1 token
5. Request allowed if tokens available

**Example:**
```
Bucket capacity: 10 tokens
Refill rate: 1 token/second

Time  Action       Tokens  Result
0s    Request #1   10→9    ✅ Allowed
0s    Request #2   9→8     ✅ Allowed
0s    Request #3   8→7     ✅ Allowed
...
0s    Request #11  0→-1    ❌ Rejected (no tokens)
1s    Refill       0→1
1s    Request #12  1→0     ✅ Allowed
```

### IP Address Detection

**Priority order for extracting client IP:**

```go
// internal/ratelimit/middleware.go:getClientIP()

1. Fly-Client-IP       // Fly.io proxy
2. CF-Connecting-IP    // Cloudflare
3. X-Real-IP           // Nginx
4. True-Client-IP      // Akamai/Cloudflare Enterprise
5. X-Forwarded-For     // Standard proxy header (first IP)
6. RemoteAddr          // Direct connection
```

**Anti-Spoofing Protection:**

Uses composite key from ALL headers:
```go
key := fmt.Sprintf("fly:%s|cf:%s|xff:%s", flyIP, cfIP, xffIP)
```

**Why composite?**
- Attacker can spoof single header
- Cannot spoof all headers consistently
- Each combination gets separate rate limit bucket

**Example:**
```
Legitimate client behind Cloudflare:
  CF-Connecting-IP: 1.2.3.4
  X-Forwarded-For: 1.2.3.4
  Key: "fly:|cf:1.2.3.4|xff:1.2.3.4"

Attacker spoofing IP:
  CF-Connecting-IP: 1.2.3.4 (real, from Cloudflare)
  X-Forwarded-For: 5.6.7.8 (spoofed)
  Key: "fly:|cf:1.2.3.4|xff:5.6.7.8"  ← Different bucket!
```

### Memory Management

**Auto-Cleanup:**

```go
// Runs every 5 minutes
func (l *InMemoryRateLimiter) cleanup() {
    ticker := time.NewTicker(cleanupInterval)
    for {
        select {
        case <-ticker.C:
            // Remove limiters with no requests in last 10 minutes
            l.limiters.Range(func(key, value interface{}) bool {
                if lastAccess < time.Now().Add(-maxAge) {
                    l.limiters.Delete(key)
                }
                return true
            })
        }
    }
}
```

**Memory usage:**
- ~32 bytes per active limiter
- 1000 concurrent IPs = ~32 KB
- 10,000 concurrent IPs = ~320 KB

**Cleanup criteria:**
- No requests in last 10 minutes
- Runs every 5 minutes
- Prevents memory leaks from one-time visitors

### Configuration

**Current (hardcoded):**
```go
// internal/api/server.go:NewServer()
globalLimiter: NewInMemoryRateLimiter(100, 200)
authLimiter: NewInMemoryRateLimiter(0.167, 5)
uploadLimiter: NewInMemoryRateLimiter(0.278, 200)
validationLimiter: NewInMemoryRateLimiter(0.5, 10)
```

**Future (configurable):**
```bash
export RATE_LIMIT_GLOBAL_RPS=100
export RATE_LIMIT_GLOBAL_BURST=200
export RATE_LIMIT_AUTH_RPM=10
export RATE_LIMIT_AUTH_BURST=5
```

### Response Headers

**Success:**
```http
HTTP/1.1 200 OK
Content-Type: application/json
...
```

**Rate Limited:**
```http
HTTP/1.1 429 Too Many Requests
Content-Type: application/json

{"error": "Rate limit exceeded. Please try again later."}
```

**Future enhancement:**
```http
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 47
X-RateLimit-Reset: 1642531200
```

### Testing Rate Limits

**Test global limiter:**
```bash
# Flood with requests
for i in {1..150}; do
  curl https://confab.dev/health &
done
wait

# Expected: First 100 succeed, rest get 429
```

**Test auth limiter:**
```bash
# Try 20 login attempts in 10 seconds
for i in {1..20}; do
  curl https://confab.dev/auth/github/login &
done
wait

# Expected: First 5 succeed, rest get 429
```

**Test with rate limiting test:**
```bash
go test -v ./internal/ratelimit/ -run TestRateLimit
```

### Scaling Considerations

**Current limitation: In-memory doesn't scale across servers**

**Problem:**
```
Server 1: Client makes 100 requests ✅ (under limit)
Server 2: Same client makes 100 requests ✅ (under limit)
Total: 200 requests (over limit!) ❌
```

**Solution: Redis-based rate limiting**

```go
// Future implementation
type RedisRateLimiter struct {
    client *redis.Client
    rate   rate.Limit
    burst  int
}

func (r *RedisRateLimiter) Allow(ctx context.Context, key string) bool {
    // Use Redis INCR + EXPIRE for distributed rate limiting
    // Or use Redis Sorted Sets for sliding window
}
```

**Benefits:**
- Shared state across all servers
- Atomic operations
- Accurate limits in multi-server setup

**Libraries:**
- `github.com/go-redis/redis_rate`
- `github.com/ulule/limiter/v3`

---

## Response Compression

### Overview

**Implementation:** Dual-encoding (Brotli + gzip)

**Purpose:** Reduce bandwidth usage and improve response times

**Savings:** 70-90% for JSON responses

**Location:** `internal/api/server.go:79-87`

### Encodings

#### Brotli (Preferred)

**Algorithm:** Brotli compression
**Level:** 5 (balanced)
**Browser Support:** 95% (Chrome 50+, Firefox 44+, Safari 10+, Edge 15+)
**Compression Ratio:** ~85% size reduction

**Pros:**
- 15-25% better compression than gzip
- Faster decompression on client
- Lower bandwidth costs

**Cons:**
- 2-3x slower compression (server CPU)
- Not supported by IE11, old curl

#### gzip (Fallback)

**Algorithm:** gzip compression
**Level:** 5 (balanced)
**Browser Support:** 100%
**Compression Ratio:** ~80% size reduction

**Pros:**
- Universal support
- Faster compression than Brotli
- Lower server CPU

**Cons:**
- Larger files than Brotli
- Older algorithm

### Implementation

```go
// internal/api/server.go
compressor := middleware.NewCompressor(5) // gzip level 5
compressor.SetEncoder("br", func(w io.Writer, level int) io.Writer {
    return brotli.NewWriterLevel(w, 5) // Brotli level 5
})
r.Use(compressor.Handler)
```

### Encoding Negotiation

**Client sends:**
```http
GET /api/v1/sessions HTTP/1.1
Accept-Encoding: gzip, br
```

**Server responds (Brotli preferred):**
```http
HTTP/1.1 200 OK
Content-Encoding: br
Vary: Accept-Encoding
Content-Type: application/json

<brotli-compressed-data>
```

**Client sends (no Brotli support):**
```http
GET /api/v1/sessions HTTP/1.1
Accept-Encoding: gzip
```

**Server responds (gzip fallback):**
```http
HTTP/1.1 200 OK
Content-Encoding: gzip
Vary: Accept-Encoding
Content-Type: application/json

<gzip-compressed-data>
```

**Client sends (no compression):**
```http
GET /api/v1/sessions HTTP/1.1
```

**Server responds (uncompressed):**
```http
HTTP/1.1 200 OK
Content-Type: application/json

<uncompressed-data>
```

### When Compression Applies

**Compressed:**
- ✅ Response size > 1KB
- ✅ Content-Type is compressible (application/json, text/*)
- ✅ Client sends Accept-Encoding header

**Not Compressed:**
- ❌ Response < 1KB (overhead not worth it)
- ❌ Already compressed (image/*, video/*)
- ❌ Client doesn't support compression
- ❌ Streaming responses

### Compression Savings

**Example: Session List (10 items)**
```
Uncompressed: 2.5 KB
gzip:         800 B  (68% reduction)
Brotli:       650 B  (74% reduction)
```

**Example: Session Detail (large)**
```
Uncompressed: 150 KB
gzip:         15 KB  (90% reduction)
Brotli:       12 KB  (92% reduction)
```

**Example: File Content (JSON)**
```
Uncompressed: 50 KB
gzip:         8 KB   (84% reduction)
Brotli:       6.5 KB (87% reduction)
```

**Bandwidth Calculator:**
```
Average API response: 10 KB uncompressed
With Brotli: ~1.5 KB (85% reduction)

100,000 requests/day:
- Without compression: 1 GB/day
- With Brotli: 150 MB/day
- Savings: 850 MB/day (85%)

Annual savings: ~310 GB
```

### Performance Impact

#### CPU Usage

**gzip level 5:**
- Compression: ~1-2ms per 100KB
- Decompression (client): ~0.5ms per 100KB

**Brotli level 5:**
- Compression: ~3-5ms per 100KB (2-3x slower)
- Decompression (client): ~0.3ms per 100KB (faster!)

#### Network Impact

**Example: 100KB JSON response over 10 Mbps connection**

```
Uncompressed:
  Transfer time: 80ms
  Total: 80ms

gzip:
  Compression: 2ms
  Transfer time (20KB): 16ms
  Total: 18ms
  Savings: 62ms ✅

Brotli:
  Compression: 5ms
  Transfer time (15KB): 12ms
  Total: 17ms
  Savings: 63ms ✅
```

**Verdict:** Extra CPU cost is negligible compared to network savings

#### Memory Usage

**Per-request overhead:**
- gzip: ~32 KB buffer
- Brotli: ~32 KB buffer

**100 concurrent requests:**
- Additional memory: ~3.2 MB
- Negligible for modern servers

### Testing Compression

**Test Brotli (preferred):**
```bash
curl -H "Accept-Encoding: br" -I https://confab.dev/api/v1/sessions
# Should see: Content-Encoding: br
```

**Test gzip (fallback):**
```bash
curl -H "Accept-Encoding: gzip" -I https://confab.dev/api/v1/sessions
# Should see: Content-Encoding: gzip
```

**Test encoding preference:**
```bash
curl -H "Accept-Encoding: gzip, br" -I https://confab.dev/api/v1/sessions
# Should see: Content-Encoding: br (Brotli preferred)
```

**Compare sizes:**
```bash
# Uncompressed
curl https://confab.dev/api/v1/sessions | wc -c

# gzip
curl -H "Accept-Encoding: gzip" --compressed \
     https://confab.dev/api/v1/sessions | wc -c

# Brotli (requires curl 7.57+)
curl -H "Accept-Encoding: br" --compressed \
     https://confab.dev/api/v1/sessions | wc -c
```

**Run compression tests:**
```bash
go test -v ./internal/api/ -run Compression
```

### Client Support

**Modern browsers (automatic):**
- ✅ Chrome 50+ (2016): Brotli + gzip
- ✅ Firefox 44+ (2016): Brotli + gzip
- ✅ Safari 10+ (2017): Brotli + gzip
- ✅ Edge 15+ (2017): Brotli + gzip

**Legacy browsers:**
- ✅ IE11: gzip only
- ✅ Older browsers: gzip only

**CLI tools:**
- ✅ curl 7.57+: Brotli + gzip (with `--compressed`)
- ✅ httpie: Automatic
- ✅ wget: gzip only

**Go http.Client:**
```go
// Automatic decompression (both gzip and Brotli)
client := &http.Client{}
resp, _ := client.Get("https://confab.dev/api/v1/sessions")
// Response body is already decompressed
```

### Configuration

**Current (hardcoded):**
```go
compressor := middleware.NewCompressor(5)
compressor.SetEncoder("br", func(w io.Writer, level int) io.Writer {
    return brotli.NewWriterLevel(w, 5)
})
```

**Future (configurable):**
```bash
export COMPRESSION_LEVEL=5        # 0-9, default 5
export COMPRESSION_MIN_SIZE=1024  # Only compress responses >1KB
export COMPRESSION_ENABLED=true   # Toggle on/off
```

---

## Database Optimization

### Connection Pooling

**Configuration:**
```go
// internal/db/db.go:Connect()
conn.SetMaxOpenConns(25)            // Max concurrent connections
conn.SetMaxIdleConns(5)             // Keep 5 ready for reuse
conn.SetConnMaxLifetime(5 * time.Minute)  // Recycle after 5 minutes
```

**Why these numbers?**
- 25 max open: Prevents overwhelming single PostgreSQL instance
- 5 idle: Balance between ready connections and resource usage
- 5 min lifetime: Prevents stale connections, allows database restarts

**Future:** Make configurable via environment variables

### Query Optimization

**Current:**
- ✅ SQL parameterization (prevents SQL injection, enables query caching)
- ✅ Indexed columns: `user_id`, `session_id`, `share_token`, `email`
- ⏳ TODO: Add EXPLAIN ANALYZE for slow queries
- ⏳ TODO: Add query timeout enforcement
- ⏳ TODO: Add connection pool monitoring

---

## Monitoring

### Metrics to Track

**Rate Limiting:**
```
rate_limit_hits_total{tier="global"}
rate_limit_hits_total{tier="auth"}
rate_limit_hits_total{tier="upload"}
rate_limit_rejections_total{tier="global"}
```

**Compression:**
```
compression_ratio{encoding="gzip"}
compression_ratio{encoding="br"}
compression_responses_total{encoding="gzip"}
compression_responses_total{encoding="br"}
uncompressed_responses_total
```

**Database:**
```
db_connections_open
db_connections_idle
db_query_duration_seconds{query="GetSession"}
```

**Request Performance:**
```
http_request_duration_seconds{path="/api/v1/sessions",method="GET"}
http_request_size_bytes
http_response_size_bytes
```

### Future: Prometheus Integration

```go
// Example metrics
var (
    httpDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "http_request_duration_seconds",
            Help: "HTTP request latency",
        },
        []string{"path", "method", "status"},
    )

    compressionRatio = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "compression_ratio",
            Help: "Compression ratio (compressed/uncompressed)",
        },
        []string{"encoding"},
    )
)
```

### Logging

**Current:**
```go
logger.Info("Processing session save",
    "user_id", userID,
    "session_id", sessionID,
    "file_count", len(files))

logger.Error("File upload failed",
    "error", err,
    "session_id", sessionID,
    "file_path", filePath)
```

**Future: Structured logging with levels**
```bash
export LOG_LEVEL=info  # debug, info, warn, error
export LOG_FORMAT=json # json, text
```

---

## Performance Checklist

### Pre-Deployment

- [ ] Rate limiting is enabled
- [ ] Compression is enabled (Brotli + gzip)
- [ ] Database connection pool is configured
- [ ] Request/response size limits are set
- [ ] HTTPS is enforced (network-level)
- [ ] CDN is configured for static assets (if applicable)

### Post-Deployment

- [ ] Monitor rate limit rejection rate
- [ ] Monitor compression ratio (target >70%)
- [ ] Monitor P99 latency (target <200ms)
- [ ] Monitor database connection pool usage
- [ ] Monitor error rates
- [ ] Load test with expected traffic

### Load Testing

```bash
# Install hey (HTTP load testing tool)
go install github.com/rakyll/hey@latest

# Test read endpoint
hey -n 10000 -c 100 https://confab.dev/health

# Test with compression
hey -n 10000 -c 100 \
    -H "Accept-Encoding: gzip, br" \
    https://confab.dev/api/v1/sessions
```

**Expected results (single server):**
- Requests/sec: >500
- P99 latency: <200ms
- Error rate: <0.1%
- Compression ratio: >70%

---

## Changelog

### 2025-01-21: Brotli Compression Added
- Added Brotli encoding (preferred over gzip)
- Dual-encoding support (automatic negotiation)
- Expected additional bandwidth savings: 15-25%
- Updated documentation to reflect Brotli + gzip

### 2025-01-21: Initial gzip Compression
- Added gzip compression middleware
- Level 5 (balanced speed/size)
- Automatic client negotiation
- Expected savings: 70-90% for JSON

### 2025-01-20: Rate Limiting Implemented
- Added 4-tier rate limiting system
- Token bucket algorithm
- In-memory storage with auto-cleanup
- IP-based and user-based limiting

### 2025-01-20: Database Connection Pooling
- Added connection pool configuration
- MaxOpenConns: 25
- MaxIdleConns: 5
- ConnMaxLifetime: 5 minutes
