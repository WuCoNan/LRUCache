package mycache

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestNewCache(t *testing.T) {
	opts := DefaultCacheOptions()
	cache := NewCache(opts)

	if cache == nil {
		t.Fatal("NewCache returned nil")
	}

	status := cache.Status()
	if status["closed"].(bool) {
		t.Error("New cache should not be closed")
	}
	if status["hits"].(uint64) != 0 {
		t.Error("New cache hits should be 0")
	}
	if status["misses"].(uint64) != 0 {
		t.Error("New cache misses should be 0")
	}
}

func TestCacheGetAndAdd(t *testing.T) {
	opts := DefaultCacheOptions()
	cache := NewCache(opts)
	defer cache.Close()

	// Test basic Add and Get
	key := "test_key"
	value := ByteView{bytes: []byte("test_value")}

	err := cache.Add(key, value)
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	got, found := cache.Get(key)
	if !found {
		t.Error("Get should find the key")
	}
	if string(got.bytes) != string(value.bytes) {
		t.Errorf("Get returned wrong value: got %s, want %s", string(got.bytes), string(value.bytes))
	}

	// Test ByteView length
	if got.Len() != len("test_value") {
		t.Errorf("ByteView length incorrect: got %d, want %d", got.Len(), len("test_value"))
	}

	// Test Get non-existent key
	_, found = cache.Get("non_existent")
	if found {
		t.Error("Get should not find non-existent key")
	}

	// Verify stats
	status := cache.Status()
	if status["hits"].(uint64) != 1 {
		t.Errorf("Expected 1 hit, got %d", status["hits"])
	}
	if status["misses"].(uint64) != 1 {
		t.Errorf("Expected 1 miss, got %d", status["misses"])
	}
}

func TestCacheAddWithExpiration(t *testing.T) {
	opts := DefaultCacheOptions()
	opts.CleanupInterval = time.Second // Set shorter cleanup for testing
	cache := NewCache(opts)
	defer cache.Close()

	key := "expiring_key"
	value := ByteView{bytes: []byte("expiring_value")}

	// Test with negative duration
	err := cache.AddWithExpiration(key, value, -time.Second)
	if err == nil {
		t.Error("AddWithExpiration should fail with negative duration")
	}

	// Test with short expiration
	err = cache.AddWithExpiration(key, value, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("AddWithExpiration failed: %v", err)
	}

	// Should find immediately
	_, found := cache.Get(key)
	if !found {
		t.Error("Should find key before expiration")
	}

	// Wait for expiration
	time.Sleep(200 * time.Millisecond)

	// Should not find after expiration
	_, found = cache.Get(key)
	if found {
		t.Error("Should not find key after expiration")
	}
}

func TestCacheDelete(t *testing.T) {
	opts := DefaultCacheOptions()
	cache := NewCache(opts)
	defer cache.Close()

	key := "to_delete"
	value := ByteView{bytes: []byte("value")}

	// Add and verify
	err := cache.Add(key, value)
	if err != nil {
		t.Fatal(err)
	}

	_, found := cache.Get(key)
	if !found {
		t.Error("Should find key before deletion")
	}

	// Delete
	err = cache.Delete(key)
	if err != nil {
		t.Fatal(err)
	}

	// Verify deletion
	_, found = cache.Get(key)
	if found {
		t.Error("Should not find key after deletion")
	}

	// Delete non-existent key (should not error)
	err = cache.Delete("non_existent")
	if err != nil {
		t.Errorf("Delete non-existent key should not error: %v", err)
	}
}

func TestCacheClear(t *testing.T) {
	opts := DefaultCacheOptions()
	cache := NewCache(opts)
	defer cache.Close()

	// Add some items
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key%d", i)
		value := ByteView{bytes: []byte(fmt.Sprintf("value%d", i))}
		cache.Add(key, value)
	}

	if cache.Len() != 10 {
		t.Errorf("Expected length 10, got %d", cache.Len())
	}

	// Clear
	err := cache.Clear()
	if err != nil {
		t.Fatal(err)
	}

	if cache.Len() != 0 {
		t.Errorf("Expected length 0 after clear, got %d", cache.Len())
	}

	// Verify all items are gone
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key%d", i)
		_, found := cache.Get(key)
		if found {
			t.Errorf("Found key %s after clear", key)
		}
	}
}

func TestCacheClose(t *testing.T) {
	opts := DefaultCacheOptions()
	cache := NewCache(opts)

	// Add an item before closing
	key := "test"
	value := ByteView{bytes: []byte("value")}
	cache.Add(key, value)

	// First close should work
	cache.Close()

	// Verify closed status
	status := cache.Status()
	if !status["closed"].(bool) {
		t.Error("Cache should be closed")
	}

	// Operations after close should fail
	_, found := cache.Get(key)
	if found {
		t.Error("Get should fail after close")
	}

	err := cache.Add("new_key", value)
	if err == nil {
		t.Error("Add should fail after close")
	}

	err = cache.Delete(key)
	if err == nil {
		t.Error("Delete should fail after close")
	}

	err = cache.Clear()
	if err == nil {
		t.Error("Clear should fail after close")
	}

	if cache.Len() != 0 {
		t.Error("Len should return 0 after close")
	}

	// Second close should be no-op (test for idempotency)
	cache.Close()
}

func TestCacheConcurrentAccess(t *testing.T) {
	opts := DefaultCacheOptions()
	cache := NewCache(opts)
	defer cache.Close()

	const goroutines = 50
	const operations = 100
	var wg sync.WaitGroup

	// Concurrent writes
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				key := fmt.Sprintf("key_%d_%d", id, j)
				value := ByteView{bytes: []byte(fmt.Sprintf("value_%d_%d", id, j))}
				cache.Add(key, value)
			}
		}(i)
	}
	wg.Wait()

	// Concurrent reads
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				key := fmt.Sprintf("key_%d_%d", id, j)
				cache.Get(key)
			}
		}(i)
	}
	wg.Wait()

	// Concurrent mixed operations
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				key := fmt.Sprintf("mixed_%d_%d", id, j)
				value := ByteView{bytes: []byte("value")}

				switch j % 4 {
				case 0:
					cache.Add(key, value)
				case 1:
					cache.Get(key)
				case 2:
					cache.Delete(key)
				case 3:
					cache.AddWithExpiration(key, value, time.Second)
				}
			}
		}(i)
	}
	wg.Wait()

	// Verify no panics occurred (if we get here, test passed)
}

func TestCacheStatus(t *testing.T) {
	opts := DefaultCacheOptions()
	cache := NewCache(opts)
	defer cache.Close()

	// Initial status
	status := cache.Status()
	if status["closed"].(bool) {
		t.Error("Cache should not be closed initially")
	}
	if _, hasHitRate := status["hit_rate"]; hasHitRate {
		t.Error("Should not have hit_rate when no operations")
	}

	// Add some hits and misses
	cache.Add("key1", ByteView{bytes: []byte("value1")})
	cache.Get("key1") // hit
	cache.Get("key1") // hit
	cache.Get("key2") // miss
	cache.Get("key3") // miss

	status = cache.Status()
	hits := status["hits"].(uint64)
	misses := status["misses"].(uint64)
	hitRate := status["hit_rate"].(float64)
	missRate := status["miss_rate"].(float64)

	if hits != 2 {
		t.Errorf("Expected 2 hits, got %d", hits)
	}
	if misses != 2 {
		t.Errorf("Expected 2 misses, got %d", misses)
	}
	if hitRate != 0.5 {
		t.Errorf("Expected hit rate 0.5, got %f", hitRate)
	}
	if missRate != 0.5 {
		t.Errorf("Expected miss rate 0.5, got %f", missRate)
	}
}

func TestCacheLen(t *testing.T) {
	opts := DefaultCacheOptions()
	cache := NewCache(opts)
	defer cache.Close()

	// Test empty cache
	if cache.Len() != 0 {
		t.Errorf("Empty cache should have length 0, got %d", cache.Len())
	}

	// Add items
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("key%d", i)
		value := ByteView{bytes: []byte(fmt.Sprintf("value%d", i))}
		cache.Add(key, value)
	}

	if cache.Len() != 5 {
		t.Errorf("Expected length 5, got %d", cache.Len())
	}

	// Delete some items
	cache.Delete("key0")
	cache.Delete("key1")

	if cache.Len() != 3 {
		t.Errorf("Expected length 3 after deletions, got %d", cache.Len())
	}

	// Clear and check
	cache.Clear()
	if cache.Len() != 0 {
		t.Errorf("Expected length 0 after clear, got %d", cache.Len())
	}
}

func TestByteView(t *testing.T) {
	opts := DefaultCacheOptions()
	cache := NewCache(opts)
	defer cache.Close()

	// Test storing empty ByteView
	empty := ByteView{}
	err := cache.Add("empty", empty)
	if err != nil {
		t.Fatalf("Failed to add empty ByteView: %v", err)
	}

	got, found := cache.Get("empty")
	if !found {
		t.Error("Should find empty ByteView")
	}
	if got.Len() != 0 {
		t.Error("Retrieved ByteView should be empty")
	}

	// Test storing nil ByteView
	nilView := ByteView{bytes: nil}
	err = cache.Add("nil", nilView)
	if err != nil {
		t.Fatalf("Failed to add nil ByteView: %v", err)
	}

	got, found = cache.Get("nil")
	if !found {
		t.Error("Should find nil ByteView")
	}
	if got.bytes != nil {
		t.Error("Retrieved ByteView should be nil")
	}
	if got.Len() != 0 {
		t.Error("Nil ByteView length should be 0")
	}
}

func TestByteViewLength(t *testing.T) {
	// Test ByteView length method
	testCases := []struct {
		name     string
		data     []byte
		expected int
	}{
		{"empty", []byte(""), 0},
		{"short", []byte("a"), 1},
		{"medium", []byte("hello world"), 11},
		{"nil", nil, 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bv := ByteView{bytes: tc.data}
			if bv.Len() != tc.expected {
				t.Errorf("ByteView.Len() = %d, want %d", bv.Len(), tc.expected)
			}
		})
	}
}

func TestDefaultCacheOptions(t *testing.T) {
	opts := DefaultCacheOptions()

	if opts.MaxBytes != 8*1024*1024 {
		t.Errorf("Default MaxBytes should be 8MB, got %d", opts.MaxBytes)
	}
	if opts.CleanupInterval != time.Minute {
		t.Errorf("Default CleanupInterval should be 1 minute, got %v", opts.CleanupInterval)
	}
	if opts.OnEvic != nil {
		t.Error("Default OnEvic should be nil")
	}
}

func TestCacheEviction(t *testing.T) {
	// This test depends on the underlying store implementation
	// You can test LRU/LFU behavior if your store supports it
	opts := DefaultCacheOptions()
	opts.MaxBytes = 1024 // Set small size to force eviction
	cache := NewCache(opts)
	defer cache.Close()

	// Add items until cache is full
	// This will depend on how your store handles eviction
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key%d", i)
		// Create a value that takes some space
		data := make([]byte, 200)
		for j := range data {
			data[j] = byte(i)
		}
		value := ByteView{bytes: data}
		cache.Add(key, value)
	}

	// The exact behavior depends on the store implementation
	// You might want to check that items were evicted
}

func TestCacheParallelGetSet(t *testing.T) {
	opts := DefaultCacheOptions()
	cache := NewCache(opts)
	defer cache.Close()

	const n = 1000
	var wg sync.WaitGroup
	wg.Add(2)

	// Concurrent setter
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			key := fmt.Sprintf("key%d", i)
			value := ByteView{bytes: []byte(fmt.Sprintf("value%d", i))}
			cache.Add(key, value)
			time.Sleep(time.Millisecond * 1)
		}
	}()

	// Concurrent getter (might get misses initially)
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			key := fmt.Sprintf("key%d", i)
			cache.Get(key)
			time.Sleep(time.Millisecond * 10)
		}
	}()

	wg.Wait()

	// Test should not panic
	status := cache.Status()
	t.Logf("Final cache status: %v", status)
}
