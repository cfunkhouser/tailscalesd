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

func (d *RateLimitedDiscoverer) refreshDevices(ctx context.Context) ([]Device, error) {
	rateLimitedRequestRefreshes.Inc()

	devices, err := d.Wrap.Devices(ctx)
	if err != nil {
		rateLimitedStaleResults.Inc()
		return devices, fmt.Errorf("%w: %w", errStaleResults, err)
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	d.last = devices
	d.earliest = time.Now().Add(d.Frequency)
	return devices, nil
}

// Devices reported by the Tailscale public API as belonging to the configured
// tailnet.
func (d *RateLimitedDiscoverer) Devices(ctx context.Context) ([]Device, error) {
	rateLimitedRequests.Inc()

	d.mu.RLock()
	expired := time.Now().After(d.earliest)
	last := make([]Device, len(d.last))
	_ = copy(last, d.last)
	d.mu.RUnlock()

	if expired {
		return d.refreshDevices(ctx)
	}
	return last, nil
}
