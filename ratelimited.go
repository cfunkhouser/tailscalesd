package tailscalesd

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var errStaleResults = errors.New("stale discovery results")

// RateLimitedDiscoverer wraps a Discoverer and limits calls to it to be no more
// frequent than once per Frequency, returning cached values if more frequent
// calls are made.
type RateLimitedDiscoverer struct {
	Wrap      Discoverer
	Frequency time.Duration

	mu       sync.RWMutex // protects following members
	earliest time.Time
	last     []Device
}

func (c *RateLimitedDiscoverer) refreshDevices(ctx context.Context) ([]Device, error) {
	rateLimitedRequestRefreshses.Inc()

	devices, err := c.Wrap.Devices(ctx)
	if err != nil {
		rateLimitedStaleResults.Inc()
		return devices, fmt.Errorf("%w: %v", errStaleResults, err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.last = devices
	c.earliest = time.Now().Add(c.Frequency)
	return devices, nil
}

func (c *RateLimitedDiscoverer) Devices(ctx context.Context) ([]Device, error) {
	rateLimitedRequests.Inc()

	c.mu.RLock()
	expired := time.Now().After(c.earliest)
	last := make([]Device, len(c.last))
	_ = copy(last, c.last)
	c.mu.RUnlock()

	if expired {
		return c.refreshDevices(ctx)
	}
	return last, nil
}
