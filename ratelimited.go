package tailscalesd

import (
	"context"
	"errors"
	"sync"
	"time"
)

var errStaleResults = errors.New("stale discovery results")

type rateLimitingDiscoverer struct {
	wrapped Discoverer
	freq    time.Duration

	mu       sync.RWMutex
	earliest time.Time
	last     []Device
}

func (c *rateLimitingDiscoverer) refreshDevices(ctx context.Context) ([]Device, error) {
	devices, err := c.wrapped.Devices(ctx)
	if err != nil {
		return devices, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.last = devices
	c.earliest = time.Now().Add(c.freq)
	return devices, nil
}

func (c *rateLimitingDiscoverer) Devices(ctx context.Context) ([]Device, error) {
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

// RateLimit requests to the API underlying client to be no more frequent than
// freq, returning cached values if more frequent calls are made.
func RateLimit(d Discoverer, freq time.Duration) Discoverer {
	return &rateLimitingDiscoverer{
		wrapped: d,
		freq:    freq,
	}
}
