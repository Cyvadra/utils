package telegram

import (
	"context"
	"sync"
	"time"
)

type limiter struct {
	mu       sync.Mutex
	limit    int
	interval time.Duration
	times    []time.Time
}

func newLimiter(limit int, interval time.Duration) *limiter {
	return &limiter{limit: limit, interval: interval}
}

func (limiter *limiter) wait(ctx context.Context) error {
	for {
		limiter.mu.Lock()
		now := time.Now()
		cutoff := now.Add(-limiter.interval)
		firstValid := 0
		for firstValid < len(limiter.times) && limiter.times[firstValid].Before(cutoff) {
			firstValid++
		}
		limiter.times = limiter.times[firstValid:]
		if len(limiter.times) < limiter.limit {
			limiter.times = append(limiter.times, now)
			limiter.mu.Unlock()
			return nil
		}
		waitUntil := limiter.times[0].Add(limiter.interval)
		limiter.mu.Unlock()

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
