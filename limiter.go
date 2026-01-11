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

// Reason represents the reason for rate limiting.
type Reason string

const (
	// ReasonFakeBot indicates the request was blocked because
	// the bot verification failed (fake bot or unknown status).
	ReasonFakeBot Reason = "fake_bot"

	// ReasonRateLimited indicates the request was blocked because
	// the IP was flagged by behavior analysis.
	ReasonRateLimited Reason = "rate_limited"
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
func New(opts ...Option) (*Limiter, error) {
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
			return nil, err
		}
		l.kb = kb
	}

	l.analyzer = analyzer.New(analyzer.Config{
		Window:        l.cfg.Window,
		PageThreshold: l.cfg.PageThreshold,
		QueueCap:      l.cfg.QueueCap,
	})

	return l, nil
}

// Allow reports whether the request should proceed.
// Returns:
//   - allowed: true if allowed, false if blocked
//   - reason: the reason for blocking when allowed is false
func (l *Limiter) Allow(ua, ip string) (allowed bool, reason Reason) {
	// Layer 1: Bot verification
	botResult := l.kb.Validate(ua, ip)

	if botResult.IsBot {
		switch botResult.Status {
		case knownbots.StatusVerified:
			// Verified bot: allow without rate limit
			return true, ""
		case knownbots.StatusPending:
			// RDNS lookup failed, allow and retry verification next time
			return true, ""
		case knownbots.StatusFailed, knownbots.StatusUnknown:
			// Fake bot (failed verification) or unknown: block immediately
			return false, ReasonFakeBot
		}
	}

	// Layer 2: Blocklist check (only for normal users)
	if l.analyzer.Blocked(ip) {
		// Behavior anomaly: apply rate limit
		if l.allowBlocked(ip) {
			return true, ""
		}
		return false, ReasonRateLimited
	}

	// Layer 3: Normal user + not blocked
	l.analyzer.Record(ip, ua)
	return true, ""
}

// Wait blocks until the request is allowed or the context is canceled.
// Returns:
//   - err: nil if allowed, otherwise the blocking error (context canceled/timeout or ErrLimit)
//   - reason: the reason for blocking (ReasonFakeBot or ReasonRateLimited)
func (l *Limiter) Wait(ctx context.Context, ua, ip string) (err error, reason Reason) {
	// Layer 1: Bot verification
	botResult := l.kb.Validate(ua, ip)

	if botResult.IsBot {
		switch botResult.Status {
		case knownbots.StatusVerified:
			// Verified bot: no rate limit needed
			return nil, ""
		case knownbots.StatusPending:
			// RDNS lookup failed, allow and retry verification next time
			return nil, ""
		case knownbots.StatusFailed, knownbots.StatusUnknown:
			// Fake bot: block immediately
			return ErrLimit, ReasonFakeBot
		}
	}

	// Layer 2: Blocklist check (only for normal users)
	if l.analyzer.Blocked(ip) {
		// Behavior anomaly: apply rate limit
		err = l.waitBlocked(ctx, ip)
		if err != nil {
			// Context canceled/timeout while waiting
			return err, ReasonRateLimited
		}
		// Rate limit hit (wait returned without error but context still active)
		return ErrLimit, ReasonRateLimited
	}

	// Layer 3: Normal user + not blocked
	l.analyzer.Record(ip, ua)
	return nil, ""
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
