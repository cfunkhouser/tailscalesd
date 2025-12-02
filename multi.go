package tailscalesd

import (
	"context"
	"errors"
	"sync"
)

// MultiDiscoverer aggregates responses from multiple Discoverers.
type MultiDiscoverer []Discoverer

type discoveryResult struct {
	devices []Device
	err     error
}

// Devices aggregates the results of calling Devices on each contained
// Discoverer. Returns the first encountered error.
func (md MultiDiscoverer) Devices(ctx context.Context) ([]Device, error) {
	multiDiscovererRequestCounter.Inc()
	var wg sync.WaitGroup
	n := len(md)
	results := make([]discoveryResult, n)
	wg.Add(n)
	for i, d := range md {
		go func(d Discoverer, result *discoveryResult) {
			defer wg.Done()
			result.devices, result.err = d.Devices(ctx)
		}(d, &results[i])
	}
	wg.Wait()

	var ret []Device
	var errs error
	for i := range results {
		if err := results[i].err; err != nil {
			multiDiscovererErrorCounter.Inc()
			errs = errors.Join(errs, err)
		}
		ret = append(ret, results[i].devices...)
	}

	return ret, errs
}
