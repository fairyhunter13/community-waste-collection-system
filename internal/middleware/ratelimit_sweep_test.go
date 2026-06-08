package middleware

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/fairyhunter13/community-waste-collection-system/internal/observability"
)

func TestSweepOnce_RemovesExpiredEntry(t *testing.T) {
	clearLimiters := func() {
		limiters.Range(func(k, v any) bool { limiters.Delete(k); return true })
	}
	clearLimiters()
	t.Cleanup(clearLimiters)

	observability.RateLimitActiveClients.Set(0)

	now := time.Now()

	expiredIP := "203.0.113.1"
	expired := &ipLimiter{}
	atomic.StoreInt64(&expired.lastSeenNano, now.Add(-2*evictThreshold).UnixNano())
	limiters.Store(expiredIP, expired)
	observability.RateLimitActiveClients.Inc()

	freshIP := "203.0.113.2"
	fresh := &ipLimiter{}
	atomic.StoreInt64(&fresh.lastSeenNano, now.UnixNano())
	limiters.Store(freshIP, fresh)
	observability.RateLimitActiveClients.Inc()

	removed := sweepOnce(now)

	assert.Equal(t, 1, removed, "only the expired entry should be removed")

	_, expiredStillPresent := limiters.Load(expiredIP)
	assert.False(t, expiredStillPresent, "expired entry must be gone")

	_, freshStillPresent := limiters.Load(freshIP)
	assert.True(t, freshStillPresent, "fresh entry must survive")
}

func TestSweepOnce_NoExpiredEntries_ReturnsZero(t *testing.T) {
	clearLimiters := func() {
		limiters.Range(func(k, v any) bool { limiters.Delete(k); return true })
	}
	clearLimiters()
	t.Cleanup(clearLimiters)

	now := time.Now()

	freshIP := "203.0.113.3"
	fresh := &ipLimiter{}
	atomic.StoreInt64(&fresh.lastSeenNano, now.UnixNano())
	limiters.Store(freshIP, fresh)

	removed := sweepOnce(now)

	assert.Equal(t, 0, removed)
	_, present := limiters.Load(freshIP)
	assert.True(t, present)
}

func TestSweepOnce_EmptyMap_ReturnsZero(t *testing.T) {
	clearLimiters := func() {
		limiters.Range(func(k, v any) bool { limiters.Delete(k); return true })
	}
	clearLimiters()
	t.Cleanup(clearLimiters)

	removed := sweepOnce(time.Now())
	assert.Equal(t, 0, removed)
}

func TestLastSeenNanoLoad_ReturnsStoredValue(t *testing.T) {
	l := &ipLimiter{}
	ts := time.Now().UnixNano()
	l.touchAndGetNano(ts)
	assert.Equal(t, ts, l.lastSeenNanoLoad())
}
