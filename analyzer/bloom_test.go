package analyzer

import (
	"testing"
)

func TestDoubleBufferBloom_New(t *testing.T) {
	bloom := NewDoubleBufferBloom()

	if bloom == nil {
		t.Fatal("NewDoubleBufferBloom() returned nil")
	}

	if bloom.current == nil {
		t.Error("current filter should be initialized")
	}

	if bloom.previous == nil {
		t.Error("previous filter should be initialized")
	}
}

func TestDoubleBufferBloom_TestAndAdd_FirstTime(t *testing.T) {
	bloom := NewDoubleBufferBloom()

	key := []byte("test-key-1")

	// First time: TestAndAdd returns false (not exists)
	result := bloom.TestAndAdd(key)
	if result {
		t.Error("first TestAndAdd should return false")
	}

	// Second time: TestAndAdd returns true (exists)
	result = bloom.TestAndAdd(key)
	if !result {
		t.Error("second TestAndAdd should return true")
	}
}

func TestDoubleBufferBloom_TestAndAdd_DifferentKeys(t *testing.T) {
	bloom := NewDoubleBufferBloom()

	keys := []string{
		"key-1",
		"key-2",
		"key-3",
		"key-4",
		"key-5",
	}

	for _, key := range keys {
		result := bloom.TestAndAdd([]byte(key))
		if result {
			t.Errorf("first TestAndAdd for %s should return false", key)
		}
	}

	// All keys should now return true
	for _, key := range keys {
		result := bloom.TestAndAdd([]byte(key))
		if !result {
			t.Errorf("second TestAndAdd for %s should return true", key)
		}
	}
}

func TestDoubleBufferBloom_TestAndAdd_DuplicateKeys(t *testing.T) {
	bloom := NewDoubleBufferBloom()

	key := []byte("duplicate-key")

	// Add same key multiple times
	for i := 0; i < 10; i++ {
		result := bloom.TestAndAdd(key)
		if i == 0 && result {
			t.Error("first insertion should return false")
		}
		if i > 0 && !result {
			t.Error("subsequent insertions should return true")
		}
	}
}

func TestDoubleBufferBloom_Rotate(t *testing.T) {
	bloom := NewDoubleBufferBloom()

	// Add some keys to current
	bloom.TestAndAdd([]byte("key-1"))
	bloom.TestAndAdd([]byte("key-2"))
	bloom.TestAndAdd([]byte("key-3"))

	// Store current filter pointer before rotation
	oldCurrent := bloom.current

	// Rotate
	bloom.Rotate()

	// After rotation:
	// - previous should be the old current
	// - current should be a new filter
	if bloom.previous != oldCurrent {
		t.Error("previous should be the old current filter")
	}

	// New keys should go to the new current
	result := bloom.TestAndAdd([]byte("key-4"))
	if result {
		t.Error("new key in rotated filter should return false")
	}
}

func TestDoubleBufferBloom_RotateMultiple(t *testing.T) {
	bloom := NewDoubleBufferBloom()

	// Multiple rotations
	for i := 0; i < 10; i++ {
		bloom.TestAndAdd([]byte("key-" + string(rune('A'+i))))
		bloom.Rotate()
	}

	// Should not panic
	_ = bloom
}

func TestDoubleBufferBloom_Consistency(t *testing.T) {
	bloom := NewDoubleBufferBloom()

	// Add many keys
	for i := 0; i < 1000; i++ {
		key := []byte("key-" + string(rune('A'+i%26)) + string(rune('0'+i/26)))
		bloom.TestAndAdd(key)
	}

	// All keys should be found
	for i := 0; i < 1000; i++ {
		key := []byte("key-" + string(rune('A'+i%26)) + string(rune('0'+i/26)))
		result := bloom.TestAndAdd(key)
		if !result {
			t.Errorf("key %d should exist", i)
		}
	}
}

func TestDoubleBufferBloom_Empty(t *testing.T) {
	bloom := NewDoubleBufferBloom()

	// Empty filter should return false for any key
	result := bloom.TestAndAdd([]byte("any-key"))
	if result {
		t.Error("empty filter should return false")
	}
}

func TestDoubleBufferBloom_ZeroKey(t *testing.T) {
	bloom := NewDoubleBufferBloom()

	// Zero key should work
	key := []byte{0, 0, 0, 0, 0, 0, 0, 0}

	result := bloom.TestAndAdd(key)
	if result {
		t.Error("zero key first insertion should return false")
	}

	result = bloom.TestAndAdd(key)
	if !result {
		t.Error("zero key second insertion should return true")
	}
}

func TestDoubleBufferBloom_LongKey(t *testing.T) {
	bloom := NewDoubleBufferBloom()

	// Long key should work
	key := []byte(string(rune('A' + 1000)))

	result := bloom.TestAndAdd(key)
	if result {
		t.Error("long key first insertion should return false")
	}

	result = bloom.TestAndAdd(key)
	if !result {
		t.Error("long key second insertion should return true")
	}
}

func BenchmarkDoubleBufferBloom_TestAndAdd(b *testing.B) {
	bloom := NewDoubleBufferBloom()
	keys := make([][]byte, b.N)
	for i := 0; i < b.N; i++ {
		keys[i] = []byte("key-" + string(rune('A'+i%26)))
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		bloom.TestAndAdd(keys[i])
	}
}

func BenchmarkDoubleBufferBloom_TestAndAddExisting(b *testing.B) {
	bloom := NewDoubleBufferBloom()

	// Pre-populate
	for i := 0; i < 10000; i++ {
		bloom.TestAndAdd([]byte("key-" + string(rune('A'+i%26))))
	}

	key := []byte("key-A")
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		bloom.TestAndAdd(key)
	}
}

func BenchmarkDoubleBufferBloom_Rotate(b *testing.B) {
	bloom := NewDoubleBufferBloom()

	// Pre-populate
	for i := 0; i < 1000; i++ {
		bloom.TestAndAdd([]byte("key-" + string(rune('A'+i%26))))
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		bloom.Rotate()
	}
}
