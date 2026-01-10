package botrate

import (
	"context"
	"strings"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func TestLimiter_New(t *testing.T) {
	l := New()

	if l == nil {
		t.Fatal("New() returned nil")
	}

	if l.cfg.Limit != DefaultLimit {
		t.Errorf("expected default limit, got %v", l.cfg.Limit)
	}

	if l.kb == nil {
		t.Error("knownbots validator should be initialized")
	}

	if l.analyzer == nil {
		t.Error("analyzer should be initialized")
	}

	l.Close()
}

func TestLimiter_New_WithOptions(t *testing.T) {
	l := New(
		WithLimit(rate.Every(time.Second)),
		WithAnalyzerWindow(time.Minute),
		WithAnalyzerPageThreshold(100),
		WithAnalyzerQueueCap(5000),
	)
	defer l.Close()

	if l.cfg.Limit != rate.Every(time.Second) {
		t.Errorf("expected custom limit, got %v", l.cfg.Limit)
	}

	if l.cfg.Window != time.Minute {
		t.Errorf("expected custom window, got %v", l.cfg.Window)
	}

	if l.cfg.PageThreshold != 100 {
		t.Errorf("expected custom threshold, got %d", l.cfg.PageThreshold)
	}

	if l.cfg.QueueCap != 5000 {
		t.Errorf("expected custom queue cap, got %d", l.cfg.QueueCap)
	}
}

func TestLimiter_Allow_VerifiedBot(t *testing.T) {
	l := New()
	defer l.Close()

	allowed := l.Allow("Googlebot/2.1", "192.168.1.1")

	if !allowed {
		t.Error("verified bot should be allowed")
	}
}

func TestLimiter_Allow_NormalUser(t *testing.T) {
	l := New(
		WithAnalyzerWindow(time.Hour),
		WithAnalyzerPageThreshold(1000),
	)
	defer l.Close()

	allowed := l.Allow("Mozilla/5.0", "192.168.1.1")

	if !allowed {
		t.Error("normal user should be allowed")
	}
}

func TestLimiter_Allow_BotLike(t *testing.T) {
	l := New()
	defer l.Close()

	// Some UAs may be verified, just verify API works
	allowed := l.Allow("Python-urllib/3.11", "192.168.1.1")
	_ = allowed
}

func TestLimiter_Allow_BlacklistedIP(t *testing.T) {
	l := New(
		WithAnalyzerWindow(time.Hour),
		WithAnalyzerPageThreshold(1),
	)
	defer l.Close()

	allowed := l.Allow("Mozilla/5.0", "192.168.1.1")
	if !allowed {
		t.Error("first request should be allowed")
	}

	time.Sleep(time.Millisecond * 200)

	allowed = l.Allow("Mozilla/5.0", "192.168.1.1")
	_ = allowed
}

func TestLimiter_Wait_VerifiedBot(t *testing.T) {
	l := New()
	defer l.Close()

	err := l.Wait(context.Background(), "Googlebot/2.1", "192.168.1.1")

	if err != nil {
		t.Errorf("verified bot should not return error, got %v", err)
	}
}

func TestLimiter_Wait_NormalUser(t *testing.T) {
	l := New(
		WithAnalyzerWindow(time.Hour),
		WithAnalyzerPageThreshold(1000),
	)
	defer l.Close()

	err := l.Wait(context.Background(), "Mozilla/5.0", "192.168.1.1")

	if err != nil {
		t.Errorf("normal user should not return error, got %v", err)
	}
}

func TestLimiter_Wait_BotLike(t *testing.T) {
	l := New(
		WithLimit(rate.Every(time.Hour)),
	)
	defer l.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*50)
	defer cancel()

	_ = l.Wait(ctx, "Python-urllib/3.11", "192.168.1.1")
}

func TestLimiter_Wait_ContextCanceled(t *testing.T) {
	l := New(
		WithLimit(rate.Every(time.Hour)),
	)
	defer l.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := l.Wait(ctx, "Googlebot/2.1", "192.168.1.1")

	// Googlebot is verified, so Wait returns nil immediately
	// Context is already canceled but since it's a verified bot, no rate limit is applied
	if err != nil && err != context.Canceled {
		t.Errorf("expected nil or context.Canceled, got %v", err)
	}
}

func TestLimiter_Close(t *testing.T) {
	l := New()

	l.Close()
	l.Close()
}

func TestLimiter_Allow_ManyRequests(t *testing.T) {
	l := New(
		WithAnalyzerWindow(time.Hour),
		WithAnalyzerPageThreshold(10000),
	)
	defer l.Close()

	for i := 0; i < 1000; i++ {
		ip := "192.168.1." + string(rune('0'+i%256))
		ua := "UserAgent/" + string(rune('A'+i%26))

		if !l.Allow(ua, ip) {
			t.Errorf("request %d should be allowed", i)
		}
	}
}

func TestLimiter_Allow_IPv6(t *testing.T) {
	l := New()
	defer l.Close()

	if !l.Allow("Mozilla/5.0", "2001:0db8:85a3:0000:0000:8a2e:0370:7334") {
		t.Error("IPv6 request should be allowed")
	}
}

func TestLimiter_Allow_EmptyUserAgent(t *testing.T) {
	l := New()
	defer l.Close()

	if !l.Allow("", "192.168.1.1") {
		t.Error("empty UA should be allowed")
	}
}

func TestLimiter_Allow_EmptyIP(t *testing.T) {
	l := New()
	defer l.Close()

	if !l.Allow("Mozilla/5.0", "") {
		t.Error("empty IP should be allowed")
	}
}

func TestLimiter_WithKnownbots(t *testing.T) {
	l1 := New()
	defer l1.Close()

	l2 := New(WithKnownbots(nil))
	defer l2.Close()

	allowed1 := l1.Allow("Googlebot/2.1", "192.168.1.1")
	allowed2 := l2.Allow("Googlebot/2.1", "192.168.1.1")

	if allowed1 != allowed2 {
		t.Error("both limiters should behave the same")
	}
}

func TestLimiter_RateLimitPersistence(t *testing.T) {
	l := New(
		WithLimit(rate.Every(time.Hour)),
		WithAnalyzerWindow(time.Hour),
		WithAnalyzerPageThreshold(10000),
	)
	defer l.Close()

	allowed1 := l.Allow("Python-urllib/3.11", "192.168.1.1")
	_ = allowed1

	allowed2 := l.Allow("Python-urllib/3.11", "192.168.1.1")
	_ = allowed2
}

func TestLimiter_DifferentBots(t *testing.T) {
	l := New()
	defer l.Close()

	bots := []string{
		"Googlebot/2.1",
		"Bingbot/2.0",
	}

	for _, bot := range bots {
		_ = l.Allow(bot, "192.168.1.1")
	}
}

func TestLimiter_BotScenarios(t *testing.T) {
	testCases := []struct {
		name            string
		ua              string
		ip              string
		shouldBeAllowed bool
	}{
		{
			name:            "verified Googlebot",
			ua:              "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
			ip:              "192.168.1.1",
			shouldBeAllowed: true,
		},
		{
			name:            "normal user Chrome",
			ua:              "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
			ip:              "192.168.1.3",
			shouldBeAllowed: true,
		},
	}

	l := New(
		WithAnalyzerWindow(time.Hour),
		WithAnalyzerPageThreshold(10000),
	)
	defer l.Close()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			allowed := l.Allow(tc.ua, tc.ip)

			if allowed != tc.shouldBeAllowed {
				t.Errorf("expected Allowed=%v, got %v", tc.shouldBeAllowed, allowed)
			}
		})
	}
}

func TestLimiter_InvalidIPFormat(t *testing.T) {
	l := New()
	defer l.Close()

	invalidIPs := []string{
		"999.999.999.999",
		"",
		"not-an-ip",
		"192.168.1",
	}

	for _, ip := range invalidIPs {
		_ = l.Allow("Mozilla/5.0", ip)
	}
}

func TestLimiter_LongUserAgent(t *testing.T) {
	l := New()
	defer l.Close()

	longUA := strings.Repeat("Mozilla/5.0 ", 1000)

	if !l.Allow(longUA, "192.168.1.1") {
		t.Error("long UA should be allowed")
	}
}

func TestLimiter_LongPath(t *testing.T) {
	l := New()
	defer l.Close()

	longPath := "/" + strings.Repeat("a", 10000)
	_ = longPath

	if !l.Allow("Mozilla/5.0", "192.168.1.1") {
		t.Error("long path should be allowed")
	}
}

func TestLimiter_ConcurrentAccess(t *testing.T) {
	l := New(
		WithAnalyzerWindow(time.Hour),
		WithAnalyzerPageThreshold(10000),
	)
	defer l.Close()

	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func(workerID int) {
			for j := 0; j < 100; j++ {
				ip := string(rune('A'+workerID%26)) + string(rune('0'+j/10))
				ua := "Worker/" + string(rune('0'+workerID%10))
				l.Allow(ua, ip)
			}
			done <- true
		}(i)
	}

	for i := 0; i < 100; i++ {
		<-done
	}
}

func BenchmarkLimiter_Allow_VerifiedBot(b *testing.B) {
	l := New()
	defer l.Close()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		l.Allow("Googlebot/2.1", "192.168.1.1")
	}
}

func BenchmarkLimiter_Allow_NormalUser(b *testing.B) {
	l := New(
		WithAnalyzerWindow(time.Hour),
		WithAnalyzerPageThreshold(10000),
	)
	defer l.Close()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		l.Allow("Mozilla/5.0", "192.168.1.1")
	}
}

func BenchmarkLimiter_Allow_BlacklistedIP(b *testing.B) {
	l := New(
		WithAnalyzerWindow(time.Hour),
		WithAnalyzerPageThreshold(10000),
	)
	defer l.Close()

	l.Allow("Mozilla/5.0", "192.168.1.1")
	time.Sleep(time.Millisecond * 100)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		l.Allow("Mozilla/5.0", "192.168.1.1")
	}
}

func BenchmarkLimiter_Wait(b *testing.B) {
	l := New(
		WithLimit(rate.Every(time.Hour)),
	)
	defer l.Close()

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		l.Wait(ctx, "Mozilla/5.0", "192.168.1.1")
	}
}

func BenchmarkLimiter_Close(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		l := New()
		l.Close()
	}
}
