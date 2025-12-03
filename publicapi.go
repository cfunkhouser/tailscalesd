package tailscalesd

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"tailscale.com/client/tailscale/v2"
)

// TailscaleAPIDiscoverer performs device discovery using the official Tailscale
// API client.
type TailscaleAPIDiscoverer struct {
	Client *tailscale.Client
}

// Devices reported by the Tailscale API as members of the tailnet.
func (d *TailscaleAPIDiscoverer) Devices(ctx context.Context) ([]Device, error) {
	start := time.Now()
	apiHost := "api.tailscale.com"
	if d.Client.BaseURL != nil {
		apiHost = d.Client.BaseURL.Host
	}
	lv := prometheus.Labels{
		"api":  "public",
		"host": apiHost,
	}
	apiRequestCounter.With(lv).Inc()
	defer func() {
		apiRequestLatencyHistogram.With(lv).Observe(float64(time.Since(start).Milliseconds()))
	}()

	devices, err := d.Client.Devices().ListWithAllFields(ctx)
	if err != nil {
		apiRequestErrorCounter.With(lv).Inc()
		return nil, err
	}

	ret := make([]Device, len(devices))
	for i := range devices {
		dev := &devices[i]
		ret[i].Addresses = dev.Addresses
		ret[i].API = apiHost
		ret[i].Authorized = dev.Authorized
		ret[i].ClientVersion = dev.ClientVersion
		ret[i].Hostname = dev.Hostname
		ret[i].ID = dev.NodeID
		ret[i].Name = dev.Name
		ret[i].OS = dev.OS
		ret[i].Tailnet = d.Client.Tailnet
		ret[i].Tags = dev.Tags
	}
	tailnetDevicesPerTailnetGauge.WithLabelValues(d.Client.Tailnet).Set(float64(len(ret)))

	return ret, nil
}
