package mycache

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/WuCoNan/MyCache/store"
)

type Cache struct {
	mu     sync.RWMutex
	store  store.Store
	hits   uint64
	misses uint64
	closed int32
}
type CacheOptions struct {
	CacheType       store.CacheType
	MaxBytes        int64
	CleanupInterval time.Duration
	OnEvic          func(key string, value store.Value)
}

func DefaultCacheOptions() CacheOptions {
	return CacheOptions{
		CacheType:       store.LRU,
		MaxBytes:        8 * 1024 * 1024,
		CleanupInterval: time.Minute,
		OnEvic:          nil,
	}
}

func NewCache(opts CacheOptions) *Cache {
	storeOptions := store.Options{
		MaxBytes:        opts.MaxBytes,
		CleanupInterval: opts.CleanupInterval,
		OnEvict:         opts.OnEvic,
	}
	return &Cache{
		store:  store.NewStore(opts.CacheType, storeOptions),
		hits:   0,
		misses: 0,
		closed: 0,
	}
}

func (c *Cache) Get(key string) (ByteView, bool) {
	if atomic.LoadInt32(&c.closed) == 1 {
		return ByteView{}, false
	}

	value, found := c.store.Get(key)
	if !found {
		atomic.AddUint64(&c.misses, 1)
		return ByteView{}, false
	}

	if byteview, ok := value.(ByteView); ok {
		atomic.AddUint64(&c.hits, 1)
		return byteview, true
	}

	fmt.Printf("Type assertion failed for key %s, expected ByteView", key)
	return ByteView{}, false
}

func (c *Cache) Add(key string, byteview ByteView) error {
	if atomic.LoadInt32(&c.closed) == 1 {
		return errors.New("cache is closed")
	}

	return c.store.Set(key, byteview)
}

func (c *Cache) AddWithExpiration(key string, byteview ByteView, duration time.Duration) error {
	if atomic.LoadInt32(&c.closed) == 1 {
		return errors.New("cache is closed")
	}

	if duration < 0 {
		return errors.New("expire duration should be positive")
	}

	return c.store.SetWithExpiration(key, byteview, duration)
}

func (c *Cache) Delete(key string) error {
	if atomic.LoadInt32(&c.closed) == 1 {
		return errors.New("cache is closed")
	}

	c.store.Delete(key)
	return nil
}

func (c *Cache) Clear() error {
	if atomic.LoadInt32(&c.closed) == 1 {
		return errors.New("cache is closed")
	}

	c.store.Clear()
	return nil
}

func (c *Cache) Len() int {
	if atomic.LoadInt32(&c.closed) == 1 {
		return 0
	}
	return c.store.Len()
}

func (c *Cache) Close() {
	if !atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.store != nil {
		c.store.Close()
		c.store = nil
	}
}

func (c *Cache) Status() map[string]interface{} {
	status := map[string]interface{}{
		"closed": atomic.LoadInt32(&c.closed) == 1,
		"hits":   atomic.LoadUint64(&c.hits),
		"misses": atomic.LoadUint64(&c.misses),
	}
	if status["hits"].(uint64)+status["misses"].(uint64) != 0 {
		status["hit_rate"] = float64(status["hits"].(uint64)) / float64(status["hits"].(uint64)+status["misses"].(uint64))
		status["miss_rate"] = float64(status["misses"].(uint64)) / float64(status["hits"].(uint64)+status["misses"].(uint64))
	}
	return status
}
