package httprate

import (
	"fmt"
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"
)

type counter struct {
	counters     map[uint64]*count
	windowLength time.Duration
	requestLimit int
	mu           sync.Mutex
}

type count struct {
	value   int
	resetAt time.Time
}

func (c *counter) Try(key string) (bool, int, time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	hkey := counterKey(key)
	now := time.Now()

	v, ok := c.counters[hkey]
	if !ok {
		v = &count{
			value:   c.requestLimit,
			resetAt: now.Add(c.windowLength),
		}
		c.counters[hkey] = v
	}

	if now.After(v.resetAt) {
		v.value = c.requestLimit
		v.resetAt = now.Add(c.windowLength)
	}

	if v.value == 0 {
		return false, 0, v.resetAt
	}
	v.value -= 1
	return true, v.value, v.resetAt
}

func (c *counter) Cleanup() {
	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.doCleanup()
		}
	}
}

func (c *counter) doCleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for k, v := range c.counters {
		if now.After(v.resetAt) {
			fmt.Printf("removed: %d", k)
			delete(c.counters, k)
		}
	}
}

func counterKey(key string) uint64 {
	h := xxhash.New()
	h.WriteString(key)
	return h.Sum64()
}
