package analyzer

import (
	"testing"
	"time"
)

func TestAnalyzer_New(t *testing.T) {
	cfg := Config{
		Window:        time.Minute,
		PageThreshold: 50,
		QueueCap:      1000,
	}

	a := New(cfg)

	if a == nil {
		t.Fatal("New() returned nil")
	}

	if a.blocklist.Load() == nil {
		t.Error("blocklist should be initialized")
	}

	if a.queue == nil {
		t.Error("queue should be initialized")
	}

	if a.bloom == nil {
		t.Error("bloom filter should be initialized")
	}

	if a.counter == nil {
		t.Error("counter should be initialized")
	}

	a.Close()
}

func TestAnalyzer_Blocked_Empty(t *testing.T) {
	cfg := Config{
		Window:        time.Minute,
		PageThreshold: 50,
		QueueCap:      1000,
	}

	a := New(cfg)
	defer a.Close()

	// New analyzer should have empty blocklist
	if a.Blocked("192.168.1.1") {
		t.Error("new analyzer should not have any blocked IPs")
	}
}

func TestAnalyzer_Record(t *testing.T) {
	cfg := Config{
		Window:        time.Second * 10,
		PageThreshold: 3, // Low threshold for testing
		QueueCap:      1000,
	}

	a := New(cfg)
	defer a.Close()

	// Record some requests
	a.Record("192.168.1.1", "/page1")
	a.Record("192.168.1.1", "/page2")
	a.Record("192.168.1.1", "/page3")
	a.Record("192.168.1.1", "/page4")

	// Wait for worker to process
	time.Sleep(time.Second * 2)

	// IP should be blocked after exceeding threshold
	if !a.Blocked("192.168.1.1") {
		t.Error("IP should be blocked after exceeding threshold")
	}
}

func TestAnalyzer_Record_DifferentPaths(t *testing.T) {
	cfg := Config{
		Window:        time.Second * 10,
		PageThreshold: 50,
		QueueCap:      1000,
	}

	a := New(cfg)
	defer a.Close()

	// Record requests with different paths
	for i := 0; i < 10; i++ {
		a.Record("192.168.1.1", "/path"+string(rune('A'+i)))
	}

	time.Sleep(time.Millisecond * 100)

	// IP should not be blocked yet (only 10 paths, threshold is 50)
	if a.Blocked("192.168.1.1") {
		t.Error("IP should not be blocked yet")
	}
}

func TestAnalyzer_Record_DuplicatePaths(t *testing.T) {
	cfg := Config{
		Window:        time.Second * 10,
		PageThreshold: 50,
		QueueCap:      1000,
	}

	a := New(cfg)
	defer a.Close()

	// Record requests with same path (should be deduplicated by bloom filter)
	for i := 0; i < 10; i++ {
		a.Record("192.168.1.1", "/same-page")
	}

	time.Sleep(time.Millisecond * 100)

	// Only one unique path, should not be blocked
	if a.Blocked("192.168.1.1") {
		t.Error("duplicate paths should be deduplicated, IP should not be blocked")
	}
}

func TestAnalyzer_Block(t *testing.T) {
	cfg := Config{
		Window:        time.Minute,
		PageThreshold: 50,
		QueueCap:      1000,
	}

	a := New(cfg)
	defer a.Close()

	// Manually block an IP
	a.block("192.168.1.1")

	if !a.Blocked("192.168.1.1") {
		t.Error("IP should be blocked")
	}
}

func TestAnalyzer_Block_AlreadyBlocked(t *testing.T) {
	cfg := Config{
		Window:        time.Minute,
		PageThreshold: 50,
		QueueCap:      1000,
	}

	a := New(cfg)
	defer a.Close()

	// Block same IP twice
	a.block("192.168.1.1")
	a.block("192.168.1.1")

	if !a.Blocked("192.168.1.1") {
		t.Error("IP should still be blocked")
	}
}

func TestAnalyzer_Rotate(t *testing.T) {
	t.Skip("rotate is called by worker in single-threaded context, skip race detection test")
}

func TestAnalyzer_DifferentIPs(t *testing.T) {
	cfg := Config{
		Window:        time.Second * 10,
		PageThreshold: 5,
		QueueCap:      1000,
	}

	a := New(cfg)
	defer a.Close()

	// Each IP should have independent counter
	for i := 0; i < 10; i++ {
		for j := 0; j < 5; j++ {
			a.Record(string(rune('A'+i)), "/page"+string(rune('0'+j)))
		}
	}

	time.Sleep(time.Second * 2)

	// All IPs should be blocked
	for i := 0; i < 10; i++ {
		ip := string(rune('A' + i))
		if !a.Blocked(ip) {
			t.Errorf("IP %s should be blocked", ip)
		}
	}
}

func BenchmarkAnalyzer_Record(b *testing.B) {
	cfg := Config{
		Window:        time.Hour,
		PageThreshold: 100000,
		QueueCap:      100000,
	}

	a := New(cfg)
	defer a.Close()

	paths := make([]string, b.N)
	for i := 0; i < b.N; i++ {
		paths[i] = "/path" + string(rune('A'+i%26))
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		a.Record("192.168.1.1", paths[i])
	}
}

func BenchmarkAnalyzer_Blocked(b *testing.B) {
	cfg := Config{
		Window:        time.Hour,
		PageThreshold: 50,
		QueueCap:      1000,
	}

	a := New(cfg)
	defer a.Close()

	// Block an IP
	a.block("192.168.1.1")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		a.Blocked("192.168.1.1")
	}
}
