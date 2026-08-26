package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWaitAllowsWithinLimit(t *testing.T) {
	limiter := New(2, time.Minute)
	ctx := context.Background()
	require.NoError(t, limiter.Wait(ctx))
	require.NoError(t, limiter.Wait(ctx))
	require.Equal(t, 2, limiter.Limit())
	require.Equal(t, time.Minute, limiter.Interval())
}

func TestWaitBlocksAtLimitUntilContextCancel(t *testing.T) {
	limiter := New(1, time.Hour)
	ctx := context.Background()
	require.NoError(t, limiter.Wait(ctx))

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	require.ErrorIs(t, limiter.Wait(cancelled), context.Canceled)
}

func TestWaitReleasesAfterInterval(t *testing.T) {
	limiter := New(1, 20*time.Millisecond)
	ctx := context.Background()
	require.NoError(t, limiter.Wait(ctx))
	time.Sleep(40 * time.Millisecond)
	require.NoError(t, limiter.Wait(ctx))
}
