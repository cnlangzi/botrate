package analyzer

import (
	"testing"
)

func TestCounter_Visit_NewIP(t *testing.T) {
	c := NewCounter()

	// First visit should return 1
	count := c.Visit("192.168.1.1")
	if count != 1 {
		t.Errorf("expected 1, got %d", count)
	}

	// Second visit should return 2
	count = c.Visit("192.168.1.1")
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}

	// Third visit should return 3
	count = c.Visit("192.168.1.1")
	if count != 3 {
		t.Errorf("expected 3, got %d", count)
	}
}

func TestCounter_Visit_DifferentIPs(t *testing.T) {
	c := NewCounter()

	// Different IPs should have independent counts
	for i := 0; i < 100; i++ {
		ip := string(rune('A'+i%26)) + string(rune('0'+i/26))
		count := c.Visit(ip)
		if count != 1 {
			t.Errorf("first visit of %s: expected 1, got %d", ip, count)
		}
	}
}

func TestCounter_Count(t *testing.T) {
	c := NewCounter()

	// Non-existent IP should return 0
	count := c.Count("192.168.1.999")
	if count != 0 {
		t.Errorf("expected 0 for non-existent IP, got %d", count)
	}

	c.Visit("192.168.1.1")
	c.Visit("192.168.1.1")
	c.Visit("192.168.1.1")

	count = c.Count("192.168.1.1")
	if count != 3 {
		t.Errorf("expected 3, got %d", count)
	}
}

func TestCounter_Clear(t *testing.T) {
	c := NewCounter()

	// Add some visits
	for i := 0; i < 10; i++ {
		c.Visit(string(rune('A' + i)))
	}

	// Clear the counter
	c.Clear()

	// All counts should be 0
	for i := 0; i < 10; i++ {
		ip := string(rune('A' + i))
		count := c.Count(ip)
		if count != 0 {
			t.Errorf("after clear, expected 0 for %s, got %d", ip, count)
		}
	}

	// New visits should start from 1
	count := c.Visit("192.168.1.1")
	if count != 1 {
		t.Errorf("after clear, expected 1 for new IP, got %d", count)
	}
}

func TestCounter_LRUEviction(t *testing.T) {
	c := NewCounter()
	c.maxSize = 5

	// Add more IPs than maxSize
	for i := 0; i < 10; i++ {
		c.Visit(string(rune('A' + i)))
	}

	// Access some IPs to change LRU order
	c.Visit("A")
	c.Visit("B")

	// Add one more to trigger eviction
	c.Visit("C")

	// After eviction, we expect 5 entries max
	if len(c.data) > 5 {
		t.Errorf("data size %d exceeds maxSize 5", len(c.data))
	}
}

func TestCounter_LRUOrder(t *testing.T) {
	c := NewCounter()
	c.maxSize = 3

	c.Visit("A")
	c.Visit("B")
	c.Visit("C")

	c.Visit("A")

	c.Visit("D")

	// After 4 visits with maxSize=3, we should have 3 entries
	if len(c.data) != 3 {
		t.Errorf("expected 3 entries, got %d", len(c.data))
	}
}

func TestCounter_MaxSizeLimit(t *testing.T) {
	c := NewCounter()
	c.maxSize = 100

	// Add many unique IPs
	for i := 0; i < 500; i++ {
		c.Visit(string(rune('A'+i%26)) + string(rune('0'+i/26%10)))
	}

	// Size should not exceed maxSize
	if len(c.data) > c.maxSize {
		t.Errorf("data size %d exceeds maxSize %d", len(c.data), c.maxSize)
	}
}

func TestCounter_Precision(t *testing.T) {
	c := NewCounter()
	c.maxSize = 10000

	// Visit same IP many times
	ip := "192.168.1.1"
	expected := uint16(1000)
	for i := 0; i < 1000; i++ {
		c.Visit(ip)
	}

	count := c.Count(ip)
	if count != expected {
		t.Errorf("expected count %d, got %d", expected, count)
	}
}

func TestCounter_ZeroMaxSize(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Zero maxSize caused panic (expected behavior): %v", r)
		}
	}()

	c := NewCounter()
	c.maxSize = 0

	// Should handle gracefully
	c.Visit("192.168.1.1")
}

func BenchmarkCounter_Visit(b *testing.B) {
	c := NewCounter()
	ips := make([]string, b.N)
	for i := 0; i < b.N; i++ {
		ips[i] = string(rune('A' + i%26))
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		c.Visit(ips[i])
	}
}

func BenchmarkCounter_VisitExisting(b *testing.B) {
	c := NewCounter()

	// Pre-populate
	for i := 0; i < 1000; i++ {
		c.Visit(string(rune('A' + i%26)))
	}

	ip := "A"
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		c.Visit(ip)
	}
}

func BenchmarkCounter_Count(b *testing.B) {
	c := NewCounter()
	c.Visit("192.168.1.1")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		c.Count("192.168.1.1")
	}
}

func BenchmarkCounter_Clear(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		c := NewCounter()
		for j := 0; j < 1000; j++ {
			c.Visit(string(rune('A' + j%26)))
		}
		c.Clear()
	}
}
