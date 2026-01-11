# botrate

<div align="center">

**High-performance, SEO-friendly rate limiter for Go applications**

[![Go Reference](https://pkg.go.dev/badge/github.com/cnlangzi/botrate.svg)](https://pkg.go.dev/github.com/cnlangzi/botrate)
[![Go Report Card](https://goreportcard.com/badge/github.com/cnlangzi/botrate)](https://goreportcard.com/report/github.com/cnlangzi/botrate)
[![codecov](https://codecov.io/gh/cnlangzi/botrate/branch/main/graph/badge.svg)](https://codecov.io/gh/cnlangzi/botrate)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

</div>

---

## Overview

BotRate is a high-performance rate limiter designed specifically for modern web applications. Unlike traditional rate limiters that blindly block high-frequency IPs, **botrate intelligently distinguishes between malicious scrapers and verified bots** (Search Engines, AI Crawlers, etc.).

This ensures your site remains protected from abuse **without sacrificing your SEO rankings** or AI knowledge base presence.

## Features

- 🛡️ **Smart Bot Detection** - Uses `knownbots` library for verified bot identification (Googlebot, Bingbot, GPTBot, ClaudeBot, etc.)
- 🔒 **Behavior Analysis** - Asynchronous IP+URL pattern detection to identify scrapers
- ⚡ **High Performance** - <2μs hot path latency, only rate limits blacklisted IPs
- 💾 **Memory Efficient** - Only creates token buckets for blacklisted IPs
- 🎯 **Flexible** - HTTP callback handling is left to caller for maximum compatibility
- 🔄 **Graceful Shutdown** - Proper resource cleanup with `Close()` method

## Installation

```bash
go get github.com/cnlangzi/botrate
```

## Quick Start

### Basic Usage

```go
package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cnlangzi/botrate"
	"golang.org/x/time/rate"
)

func main() {
	limiter := botrate.New(
		// Rate limiting for blacklisted IPs only
		botrate.WithLimit(rate.Every(10*time.Minute)),

		// Behavior analysis
		botrate.WithAnalyzerWindow(time.Minute),
		botrate.WithAnalyzerPageThreshold(50),
		botrate.WithAnalyzerQueueCap(10000),
	)
	defer limiter.Close()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua := r.UserAgent()
		ip := extractIP(r)

		result := limiter.Allow(ua, ip)
		if !result.Allowed {
			switch result.Reason {
			case "fake bot":
				http.Error(w, "Bot verification failed", http.StatusForbidden)
			case "rate limited":
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			default:
				http.Error(w, "Forbidden", http.StatusForbidden)
			}
			return
		}

		w.Write([]byte("Hello, World!"))
	})

	fmt.Println("Server started on :8080")
	http.Handle("/", handler)
	http.ListenAndServe(":8080", nil)
}

// extractIP extracts the real client IP from the request.
func extractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	return r.RemoteAddr
}
```

### Using Wait Method (Blocking)

For scenarios where you want to wait instead of immediately rejecting:

```go
handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if err := limiter.Wait(r.Context(), ua, ip); err != nil {
		http.Error(w, err.Error(), http.StatusTooManyRequests)
		return
	}
	w.Write([]byte("Hello!"))
})
```

### Middleware Pattern

```go
func BotRateMiddleware(l *botrate.Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !l.Allow(r.UserAgent(), extractIP(r)) {
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Usage
http.Handle("/", BotRateMiddleware(limiter)(myHandler))
```

## API Reference

### Options

| Option | Description | Default |
|--------|-------------|---------|
| `WithLimit(rate.Limit)` | Requests per second for blocked IPs | `rate.Every(10*time.Minute)` |
| `WithAnalyzerWindow(time.Duration)` | Analysis window duration | `5*time.Minute` |
| `WithAnalyzerPageThreshold(int)` | Max distinct pages threshold | `50` |
| `WithAnalyzerQueueCap(int)` | Event queue capacity | `10000` |
| `WithKnownbots(*knownbots.Validator)` | Custom knownbots validator | `nil` (use default) |

### Methods

#### `Allow(ua, ip string) bool`

Non-blocking check if the request should proceed. Returns `true` if allowed, `false` if blocked.

**Bot Detection Logic:**
- **Verified bot** (StatusVerified): ✅ Allow immediately
- **RDNS lookup failed** (StatusPending): ✅ Allow, retry verification next time
- **Fake bot** (StatusFailed): ❌ Block immediately
- **Normal user**: Continue to analyzer and blocklist check

```go
allowed := limiter.Allow(ua, ip)
if !allowed {
    // Request was blocked (fake bot or blacklisted IP)
}
```
result := limiter.Allow(ua, ip)
if !result.Allowed {
	// Handle denial
}
```

#### `Wait(ctx context.Context, ua, ip string) error`

Blocks until the request is allowed or the context ends. Returns `nil` if allowed, `ErrLimit` if blocked.

**Bot Detection Logic:**
- **Verified bot** (StatusVerified): ✅ Allow immediately
- **RDNS lookup failed** (StatusPending): ✅ Allow, retry verification next time
- **Fake bot** (StatusFailed): ❌ Block immediately
- **Normal user**: Continue to analyzer and blocklist check

```go
err := limiter.Wait(ctx, ua, ip)
if err != nil {
    // Handle denial (ErrLimit) or context cancellation
}
```

#### `Close()`

Gracefully shuts down the limiter and releases resources. **Always call this when the limiter is no longer needed.**

```go
limiter := botrate.New(...)
defer limiter.Close()
```

## How It Works

```
Request
  │
  ▼
┌─────────────────────────────────────┐
│ 1. KnownBots Verification           │  Hot path: <1μs
│    - Check if UA matches known bot  │
│    - Verified → Allow immediately   │
│    - RDNS failed → Allow, retry     │
│    - Fake bot → Block immediately   │
└─────────────────────────────────────┘
  │
  ▼ (only for normal users)
┌─────────────────────────────────────┐
│ 2. Blocklist Check                  │  Atomic read: <100ns
│    - Check if IP is blacklisted     │
│    - Not blocked → Record + Allow   │
│    - Blocked → Rate limit           │
└─────────────────────────────────────┘
  │
  ▼ (async)
┌─────────────────────────────────────┐
│ 3. Behavior Analysis                │  Background worker
│    - Record IP+URL combination      │
│    - Bloom filter deduplication     │
│    - Visit counter increment        │
│    - Threshold exceeded → Block     │
└─────────────────────────────────────┘
```

### Key Design Decisions

1. **Fake bots blocked immediately** - Known bot UAs with failed verification are blocked without rate limiting
2. **RDNS lookup failures are tolerated** - Failed DNS lookups allow the request (will retry next time)
3. **Verified bots bypass everything** - Googlebot, Bingbot, etc. are allowed without rate limiting
4. **Normal users go through analyzer** - Behavior analysis only applies to regular users
5. **Async behavior analysis** - Request processing is never blocked by analysis

## Performance

| Scenario | Latency | Memory |
|----------|---------|--------|
| **Normal user** | <1.5μs | 0 bytes |
| **Verified bot** | <1μs | 0 bytes |
| **Blacklisted IP** | <2μs | ~200 bytes/IP |
| **Fake bot** | <1μs | 0 bytes |

**Total memory budget**: <5MB (Bloom: 1MB + Counter: 1MB + Blacklisted IPs: variable)

### Benchmark Results

```bash
$ go test -run=^$ -bench=. -benchmem -cpu=1,4,8
```

Key metrics to monitor:
- **ns/op** - Nanoseconds per operation (lower is better)
- **B/op** - Bytes allocated per operation (should be 0 for hot path)
- **allocs/op** - Allocations per operation (should be 0 for hot path)

## Error Handling

### Errors

```go
var ErrLimit = context.DeadlineExceeded
```

Check errors with:

```go
if errors.Is(err, botrate.ErrLimit) {
    // Request was denied (fake bot or blacklisted IP)
}
```

### Denial Reasons

`Allow()` returns `false` when:

1. **Fake bot** - Known bot UA (e.g., "GPTBot") but IP verification failed
2. **Blacklisted IP** - IP was flagged by behavior analysis

`Wait()` returns `ErrLimit` when:

1. **Fake bot** - Blocked immediately
2. **Rate limited** - Normal user on blocklist hitting rate limit

```go
allowed := limiter.Allow(ua, ip)

if !allowed {
    // Request was denied
    // - Fake bot: blocked immediately
    // - Blacklisted IP: rate limited
}
```

## Configuration Examples

### Strict Rate Limiting

```go
limiter := botrate.New(
	botrate.WithLimit(rate.Every(10*time.Minute)),
)
```

### Aggressive Bot Detection

```go
limiter := botrate.New(
	botrate.WithAnalyzerWindow(30*time.Second),
	botrate.WithAnalyzerPageThreshold(20),
)
```

### High-Throughput Configuration

```go
limiter := botrate.New(
	botrate.WithAnalyzerWindow(10*time.Minute),
	botrate.WithAnalyzerPageThreshold(100),
	botrate.WithAnalyzerQueueCap(50000),
)
```

### Custom KnownBots Validator

```go
// Create custom validator with specific configuration
customKB := knownbots.New(
	knownbots.WithRoot("./custom-bots"),
	knownbots.WithSchedulerInterval(12*time.Hour),
)

// Use custom validator
limiter := botrate.New(
	botrate.WithKnownbots(customKB),
	botrate.WithLimit(rate.Every(5*time.Minute)),
)
```

## Architecture

```
botrate/
├── limiter.go          # Main Limiter type and API
├── botrate.go          # Error definitions
├── config.go           # Configuration struct
├── options.go          # Functional options
├── analyzer/           # Behavior analysis engine
│   ├── analyzer.go    # Core analyzer with worker
│   ├── bloom.go       # Double-buffered Bloom filter
│   └── counter.go     # LRU visit counter (O(1))
└── example/
    └── main.go        # Working example
```

## Development

### Makefile Commands

A Makefile is provided for common development tasks:

```bash
make help          # Show available commands
make test          # Run all tests (short + race)
make test-short    # Run short tests (fast)
make test-race     # Run tests with race detector
make test-coverage # Run tests with coverage report
make bench         # Run benchmarks (1 and 4 CPUs)
make bench-all     # Run all benchmarks (1, 4, 8 CPUs)
make build         # Build the project
make clean         # Clean build artifacts
```

### Examples

```bash
# Run all tests
make test

# Run benchmarks
make bench

# Generate coverage report
make test-coverage

# Run benchmarks with detailed output
make bench-all
```

## Contributing

Contributions are welcome! Please read our contributing guidelines before submitting PRs.

1. Fork the repository
2. Create a feature branch
3. Add tests for your changes
4. Ensure all tests pass: `make test`
5. Run benchmarks: `make bench`
6. Submit a pull request

## License

MIT License - see [LICENSE](LICENSE) for details.

## Acknowledgments

- [knownbots](https://github.com/cnlangzi/knownbots) - Bot detection library
- [bloom](https://github.com/bits-and-blooms/bloom/v3) - Bloom filter implementation
- [golang/x/time](https://pkg.go.dev/golang.org/x/time/rate) - Token bucket rate limiter
