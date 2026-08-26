// Package ratelimit provides a concurrency-safe sliding-window rate limiter
// shared by the notification integrations in this module.
package ratelimit

import (
	"context"
	"sync"
	"time"
)

// Limiter permits at most Limit requests per Interval using a sliding window.
// It is safe for concurrent use.
type Limiter struct {
	mu       sync.Mutex
	limit    int
	interval time.Duration
	times    []time.Time
}

// New returns a Limiter that allows at most limit requests per interval.
func New(limit int, interval time.Duration) *Limiter {
	return &Limiter{limit: limit, interval: interval}
}

// Limit returns the maximum number of requests allowed per interval.
func (l *Limiter) Limit() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.limit
}

// Interval returns the length of the sliding window.
func (l *Limiter) Interval() time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.interval
}

// Wait blocks until capacity is available, records a slot, and returns nil.
// It returns ctx.Err() if ctx is cancelled while waiting.
func (l *Limiter) Wait(ctx context.Context) error {
	for {
		l.mu.Lock()
		now := time.Now()
		cutoff := now.Add(-l.interval)
		firstValid := 0
		for firstValid < len(l.times) && l.times[firstValid].Before(cutoff) {
			firstValid++
		}
		l.times = l.times[firstValid:]

		if len(l.times) < l.limit {
			l.times = append(l.times, now)
			l.mu.Unlock()
			return nil
		}

		waitUntil := l.times[0].Add(l.interval)
		l.mu.Unlock()

		timer := time.NewTimer(time.Until(waitUntil))
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}
