package botrate

import (
	"context"
	"sync"
	"time"

	"github.com/cnlangzi/botrate/analyzer"
	"github.com/cnlangzi/knownbots"
	"golang.org/x/time/rate"
)

// Default configuration values.
var (
	DefaultLimit         = rate.Every(10 * time.Minute) // Very strict: 1 request per 10 min
	DefaultWindow        = 5 * time.Minute
	DefaultPageThreshold = 50
	DefaultQueueCap      = 10000
)

// Limiter provides bot-aware rate limiting.
type Limiter struct {
	cfg Config

	// Token bucket limiters (only for blocked IPs)
	blocked sync.Map

	// KnownBots validator (can be customized via option)
	kb *knownbots.Validator

	// Behavior analyzer (always enabled)
	analyzer *analyzer.Analyzer
}

// New creates a new rate limiter with default config and applies options.
func New(opts ...Option) *Limiter {
	l := &Limiter{
		cfg: Config{
			Limit:         DefaultLimit,
			Window:        DefaultWindow,
			PageThreshold: DefaultPageThreshold,
			QueueCap:      DefaultQueueCap,
		},
	}

	for _, opt := range opts {
		opt(l)
	}

	if l.kb == nil {
		kb, err := knownbots.New()
		if err != nil {
			panic(err)
		}
		l.kb = kb
	}

	l.analyzer = analyzer.New(analyzer.Config{
		Window:        l.cfg.Window,
		PageThreshold: l.cfg.PageThreshold,
		QueueCap:      l.cfg.QueueCap,
	})

	return l
}

// Allow reports whether the request should proceed.
// Returns true if allowed, false if rate limited.
func (l *Limiter) Allow(ua, ip string) bool {
	// Layer 1: Bot verification
	botResult := l.kb.Validate(ua, ip)

	if botResult.IsBot {
		if botResult.Status == knownbots.StatusVerified {
			// Verified bot: allow without rate limit
			return true
		}
		// Fake bot: apply rate limit
		return l.allowBlocked(ip)
	}

	// Layer 2: Blocklist check (only for normal users)
	if l.analyzer.Blocked(ip) {
		// Behavior anomaly: apply rate limit
		return l.allowBlocked(ip)
	}

	// Layer 3: Normal user + not blocked
	l.analyzer.Record(ip, ua)
	return true
}

// Wait blocks until the request is allowed or the context is canceled.
func (l *Limiter) Wait(ctx context.Context, ua, ip string) error {
	// Layer 1: Bot verification
	botResult := l.kb.Validate(ua, ip)

	if botResult.IsBot {
		if botResult.Status == knownbots.StatusVerified {
			// Verified bot: no rate limit needed
			return nil
		}
		// Fake bot: apply rate limit
		return l.waitBlocked(ctx, ip)
	}

	// Layer 2: Blocklist check (only for normal users)
	if l.analyzer.Blocked(ip) {
		// Behavior anomaly: apply rate limit
		return l.waitBlocked(ctx, ip)
	}

	// Layer 3: Normal user + not blocked
	l.analyzer.Record(ip, ua)
	return nil
}

func (l *Limiter) allowBlocked(ip string) bool {
	limiter := l.getLimiter(ip)
	return limiter.Allow()
}

func (l *Limiter) waitBlocked(ctx context.Context, ip string) error {
	limiter := l.getLimiter(ip)
	return limiter.Wait(ctx)
}

func (l *Limiter) getLimiter(ip string) *rate.Limiter {
	if val, ok := l.blocked.Load(ip); ok {
		return val.(*rate.Limiter)
	}
	limiter := rate.NewLimiter(l.cfg.Limit, 1) // Burst=1 for strict blocking
	actual, _ := l.blocked.LoadOrStore(ip, limiter)
	return actual.(*rate.Limiter)
}

// Close gracefully shuts down the limiter and releases resources.
func (l *Limiter) Close() {
	l.analyzer.Close()
	
	l.blocked.Range(func(key, value any) bool {
		l.blocked.Delete(key)
		return true
	})
}
