package tailscalesd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// LocalAPISocket is the path to the Unix domain socket on which tailscaled
// listens locally.
const LocalAPISocket = "/run/tailscale/tailscaled.sock"

// interstingStatusSubset is a json-decodeable subset of the Status struct
// served by the Tailscale local API. This is done to prevent pulling the
// Tailscale code base and its dependencies into this module. The fields were
// borrowed from version 1.22.2. For field details, see:
// https://pkg.go.dev/tailscale.com@v1.22.2/ipn/ipnstate?utm_source=gopls#Status
type interestingStatusSubset struct {
	TailscaleIPs []netip.Addr // Tailscale IP(s) assigned to this node
	Self         *interestingPeerStatusSubset
	Peer         map[string]*interestingPeerStatusSubset
}

// interestingPeerStatusSubset is the PeerStatus equivalent of
// interestingStatusSubset.
type interestingPeerStatusSubset struct {
	ID           string
	HostName     string
	DNSName      string
	OS           string
	TailscaleIPs []netip.Addr
	Tags         []string `json:",omitempty"`
}

type localAPIClient struct {
	client *http.Client
}

var errFailedLocalAPIRequest = errors.New("failed local API request")

func (a *localAPIClient) status(ctx context.Context) (interestingStatusSubset, error) {
	start := time.Now()
	lv := prometheus.Labels{
		"api":  "local",
		"host": "localhost",
	}
	defer func() {
		apiRequestLatencyHistogram.With(lv).Observe(float64(time.Since(start).Milliseconds()))
	}()

	var status interestingStatusSubset
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://local-tailscaled.sock/localapi/v0/status", nil)
	if err != nil {
		return status, err
	}

	apiRequestCounter.With(lv).Inc()
	resp, err := a.client.Do(req)
	if err != nil {
		apiRequestErrorCounter.With(lv).Inc()
		return status, err
	}
	if (resp.StatusCode / 100) != 2 {
		apiRequestErrorCounter.With(lv).Inc()
		return status, fmt.Errorf("%w: %v", errFailedLocalAPIRequest, resp.Status)
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		apiPayloadErrorCounter.With(lv).Inc()
		return status, err
	}
	return status, nil
}

func translatePeerToDevice(p *interestingPeerStatusSubset, d *Device) {
	for i := range p.TailscaleIPs {
		d.Addresses = append(d.Addresses, p.TailscaleIPs[i].String())
	}
	d.API = "localhost"
	d.Authorized = true // localapi returned peer; assume it's authorized enough
	d.Hostname = p.HostName
	d.ID = p.ID
	d.OS = p.OS
	d.Tags = p.Tags[:]
}

// Devices reported by the Tailscale local API as peers of the local host.
func (a *localAPIClient) Devices(ctx context.Context) ([]Device, error) {
	status, err := a.status(ctx)
	if err != nil {
		return nil, err
	}
	devices := make([]Device, len(status.Peer))
	var i int
	for _, peer := range status.Peer {
		translatePeerToDevice(peer, &devices[i])
		i++
	}
	return devices, nil
}

type dialContext func(context.Context, string, string) (net.Conn, error)

func unixSocketDialer(socket string) dialContext {
	return func(ctx context.Context, _, _ string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, "unix", socket)
	}
}

func defaultHTTPClientWithDialer(dc dialContext) *http.Client {
	return &http.Client{
		Timeout: time.Second * 10,
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout: 5 * time.Second,
			}).Dial,
			DialContext:         dc,
			TLSHandshakeTimeout: 5 * time.Second,
		},
	}
}

// LocalAPI Discoverer interrogates the Tailscale localapi for peer devices.
func LocalAPI(socket string) Discoverer {
	return &localAPIClient{
		client: defaultHTTPClientWithDialer(unixSocketDialer(socket)),
	}
}
