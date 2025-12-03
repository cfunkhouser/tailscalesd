package tailscalesd

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"tailscale.com/client/local"
	"tailscale.com/ipn/ipnstate"
)

// LocalAPIDiscoverer discovers devices from the API served by the Tailscale
// daemon on the local machine.
type LocalAPIDiscoverer struct {
	Client local.Client
}

func peerToDevice(p *ipnstate.PeerStatus, d *Device) {
	for i := range p.TailscaleIPs {
		d.Addresses = append(d.Addresses, p.TailscaleIPs[i].String())
	}
	d.API = "localhost"
	d.Authorized = true // localapi returned peer; assume it's authorized enough
	d.Hostname = p.HostName
	d.ID = string(p.ID)
	d.OS = p.OS
	if p.Tags != nil {
		d.Tags = p.Tags.AsSlice()
	}
}

// Devices reported by the Tailscale local API as peers of the local host.
func (d *LocalAPIDiscoverer) Devices(ctx context.Context) ([]Device, error) {
	start := time.Now()
	lv := prometheus.Labels{
		"api":  "local",
		"host": "localhost",
	}
	apiRequestCounter.With(lv).Inc()
	defer func() {
		apiRequestLatencyHistogram.With(lv).Observe(float64(time.Since(start).Milliseconds()))
	}()

	status, err := d.Client.Status(ctx)
	if err != nil {
		apiRequestErrorCounter.With(lv).Inc()
		return nil, err
	}

	ret := make([]Device, len(status.Peer))
	var i int
	for _, peer := range status.Peer {
		peerToDevice(peer, &ret[i])
		i++
	}

	return ret, nil
}

// LocalAPI Discoverer interrogates the Tailscale localapi for peer devices.
func LocalAPI(socket string) *LocalAPIDiscoverer {
	var ret LocalAPIDiscoverer
	ret.Client.Socket = socket

	return &ret
}
