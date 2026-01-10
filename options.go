package botrate

import (
	"time"

	"github.com/cnlangzi/knownbots"
	"golang.org/x/time/rate"
)

// Option is a functional option for configuring Limiter.
type Option func(*Limiter)

// WithLimit sets events per second for rate limiting.
func WithLimit(limit rate.Limit) Option {
	return func(l *Limiter) {
		l.cfg.Limit = limit
	}
}

// WithAnalyzerWindow sets analysis window duration.
func WithAnalyzerWindow(window time.Duration) Option {
	return func(l *Limiter) {
		l.cfg.Window = window
	}
}

// WithAnalyzerPageThreshold sets max distinct pages threshold.
func WithAnalyzerPageThreshold(threshold int) Option {
	return func(l *Limiter) {
		l.cfg.PageThreshold = threshold
	}
}

// WithAnalyzerQueueCap sets event queue capacity.
func WithAnalyzerQueueCap(cap int) Option {
	return func(l *Limiter) {
		l.cfg.QueueCap = cap
	}
}

// WithKnownbots implants a custom knownbots.Validator.
func WithKnownbots(kb *knownbots.Validator) Option {
	return func(l *Limiter) {
		l.kb = kb
	}
}
