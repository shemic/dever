package cache

import (
	"sync"
	"sync/atomic"
	"time"
)

type Option func(*options)

type options struct {
	ttl             time.Duration
	maxEntries      int
	cleanupInterval uint64
}

type entry[V any] struct {
	value     V
	epoch     uint64
	expiresAt time.Time
}

type loadCall[V any] struct {
	wg    sync.WaitGroup
	value V
	err   error
}

type VersionedCache[K comparable, V any] struct {
	mu       sync.RWMutex
	items    map[K]entry[V]
	inflight map[K]*loadCall[V]
	epoch    atomic.Uint64
	writes   atomic.Uint64
	opts     options
}

func New[K comparable, V any](opts ...Option) *VersionedCache[K, V] {
	config := options{
		maxEntries:      1024,
		cleanupInterval: 128,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&config)
		}
	}
	if config.cleanupInterval == 0 {
		config.cleanupInterval = 128
	}
	return &VersionedCache[K, V]{
		items: make(map[K]entry[V]),
		opts:  config,
	}
}

func WithTTL(ttl time.Duration) Option {
	return func(opts *options) {
		opts.ttl = ttl
	}
}

func WithMaxEntries(maxEntries int) Option {
	return func(opts *options) {
		opts.maxEntries = maxEntries
	}
}

func WithCleanupInterval(interval uint64) Option {
	return func(opts *options) {
		opts.cleanupInterval = interval
	}
}

func (c *VersionedCache[K, V]) Load(key K) (V, bool) {
	if c == nil {
		var zero V
		return zero, false
	}

	currentEpoch := c.Epoch()
	now := time.Now()

	c.mu.RLock()
	item, ok := c.items[key]
	c.mu.RUnlock()
	if !ok {
		var zero V
		return zero, false
	}

	if c.entryExpired(item, currentEpoch, now) {
		c.deleteExpiredEntry(key, item, currentEpoch, now)
		var zero V
		return zero, false
	}

	return item.value, true
}

func (c *VersionedCache[K, V]) Store(key K, value V) {
	if c == nil {
		return
	}
	c.storeAtEpoch(key, value, c.Epoch())
}

func (c *VersionedCache[K, V]) GetOrSet(key K, loader func() (V, error)) (V, error) {
	if value, ok := c.Load(key); ok {
		return value, nil
	}
	if loader == nil {
		var zero V
		return zero, nil
	}

	startEpoch := c.Epoch()
	call, leader := c.beginLoad(key)
	if !leader {
		call.wg.Wait()
		return call.value, call.err
	}
	defer c.finishLoad(key, call)

	if value, ok := c.Load(key); ok {
		call.value = value
		return value, nil
	}

	call.value, call.err = loader()
	if call.err == nil && c.Epoch() == startEpoch {
		c.storeAtEpoch(key, call.value, startEpoch)
	}
	return call.value, call.err
}

func (c *VersionedCache[K, V]) storeAtEpoch(key K, value V, epoch uint64) {
	item := entry[V]{
		value: value,
		epoch: epoch,
	}
	if c.opts.ttl > 0 {
		item.expiresAt = time.Now().Add(c.opts.ttl)
	}

	c.mu.Lock()
	if c.items == nil {
		c.items = make(map[K]entry[V])
	}
	c.items[key] = item
	length := len(c.items)
	c.mu.Unlock()

	writeCount := c.writes.Add(1)
	if c.shouldCleanup(length, writeCount) {
		c.Cleanup()
	}
}

func (c *VersionedCache[K, V]) Invalidate() {
	if c == nil {
		return
	}
	c.epoch.Add(1)
}

func (c *VersionedCache[K, V]) Clear() {
	if c == nil {
		return
	}
	c.epoch.Add(1)
	c.mu.Lock()
	c.items = make(map[K]entry[V])
	c.mu.Unlock()
}

func (c *VersionedCache[K, V]) Cleanup() {
	if c == nil {
		return
	}

	currentEpoch := c.Epoch()
	now := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	for key, item := range c.items {
		if c.entryExpired(item, currentEpoch, now) {
			delete(c.items, key)
		}
	}

	if c.opts.maxEntries <= 0 {
		return
	}
	for key := range c.items {
		if len(c.items) <= c.opts.maxEntries {
			break
		}
		delete(c.items, key)
	}
}

func (c *VersionedCache[K, V]) Len() int {
	if c == nil {
		return 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

func (c *VersionedCache[K, V]) Epoch() uint64 {
	if c == nil {
		return 0
	}
	return c.epoch.Load()
}

func (c *VersionedCache[K, V]) shouldCleanup(length int, writeCount uint64) bool {
	if c.opts.maxEntries > 0 && length > c.opts.maxEntries {
		return true
	}
	return writeCount%c.opts.cleanupInterval == 0
}

func (c *VersionedCache[K, V]) entryExpired(item entry[V], epoch uint64, now time.Time) bool {
	if item.epoch != epoch {
		return true
	}
	return !item.expiresAt.IsZero() && !item.expiresAt.After(now)
}

func (c *VersionedCache[K, V]) deleteExpiredEntry(key K, item entry[V], epoch uint64, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	current, ok := c.items[key]
	if !ok {
		return
	}
	if current.epoch != item.epoch || !current.expiresAt.Equal(item.expiresAt) {
		return
	}
	if c.entryExpired(current, epoch, now) {
		delete(c.items, key)
	}
}

func (c *VersionedCache[K, V]) beginLoad(key K) (*loadCall[V], bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.inflight == nil {
		c.inflight = make(map[K]*loadCall[V])
	}
	if call, ok := c.inflight[key]; ok {
		return call, false
	}

	call := &loadCall[V]{}
	call.wg.Add(1)
	c.inflight[key] = call
	return call, true
}

func (c *VersionedCache[K, V]) finishLoad(key K, call *loadCall[V]) {
	call.wg.Done()

	c.mu.Lock()
	defer c.mu.Unlock()
	if current := c.inflight[key]; current == call {
		delete(c.inflight, key)
	}
}
