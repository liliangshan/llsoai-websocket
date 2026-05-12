package guard

import (
	"container/list"
	"sync"
	"time"
)

type RateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	entries map[string][]time.Time
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{limit: limit, window: window, entries: map[string][]time.Time{}}
}

func (r *RateLimiter) Allow(key string) bool {
	if r == nil || r.limit <= 0 || key == "" {
		return true
	}
	now := time.Now()
	cutoff := now.Add(-r.window)
	r.mu.Lock()
	defer r.mu.Unlock()
	items := r.entries[key]
	kept := items[:0]
	for _, item := range items {
		if item.After(cutoff) {
			kept = append(kept, item)
		}
	}
	if len(kept) >= r.limit {
		r.entries[key] = kept
		return false
	}
	kept = append(kept, now)
	r.entries[key] = kept
	return true
}

type Deduper struct {
	mu       sync.Mutex
	ttl      time.Duration
	max      int
	items    map[string]time.Time
	ordering *list.List
}

func NewDeduper(max int, ttl time.Duration) *Deduper {
	return &Deduper{max: max, ttl: ttl, items: map[string]time.Time{}, ordering: list.New()}
}

func (d *Deduper) Seen(key string) bool {
	if d == nil || key == "" {
		return false
	}
	now := time.Now()
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cleanup(now)
	if expires, ok := d.items[key]; ok && expires.After(now) {
		return true
	}
	d.items[key] = now.Add(d.ttl)
	d.ordering.PushBack(key)
	for len(d.items) > d.max && d.ordering.Len() > 0 {
		front := d.ordering.Front()
		value, _ := front.Value.(string)
		delete(d.items, value)
		d.ordering.Remove(front)
	}
	return false
}

func (d *Deduper) cleanup(now time.Time) {
	for key, expires := range d.items {
		if now.After(expires) {
			delete(d.items, key)
		}
	}
}
