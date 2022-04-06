package tailscalesd

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"

	"inet.af/netaddr"
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
	TailscaleIPs []netaddr.IP // Tailscale IP(s) assigned to this node
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
	TailscaleIPs []netaddr.IP
	Tags         []string `json:",omitempty"`
}

type localAPIClient struct {
	client *http.Client
	socket string
}

func (a *localAPIClient) status(ctx context.Context) (interestingStatusSubset, error) {
	var status interestingStatusSubset
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost/localapi/v0/status", nil)
	if err != nil {
		return status, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return status, err
	}

	if (resp.StatusCode / 100) != 2 {
		return status, fmt.Errorf("%w: %v", errFailedRequest, resp.Status)
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return status, err
	}
	return status, nil
}

func translatePeerToDevice(p *interestingPeerStatusSubset, d *Device) {
	for i := range p.TailscaleIPs {
		d.Addresses = append(d.Addresses, p.TailscaleIPs[i].String())
	}

	// Assumes that if the peer is listed in localapi, it is authorized enough.
	d.Authorized = true
	d.API = "localhost"
	d.Hostname = p.HostName
	d.ID = fmt.Sprintf("%v", p.ID)
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

// unixDialer is a DialContext allowing HTTP communication via a unix  domain
// socket.
func (a *localAPIClient) unixDialer(ctx context.Context, _, _ string) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, "unix", a.socket)
}

// LocalAPI Discoverer interrogates the Tailscale localapi for peer devices.
func LocalAPI(socket string) Discoverer {
	api := &localAPIClient{
		socket: socket,
	}
	api.client = &http.Client{
		Transport: &http.Transport{
			DialContext: api.unixDialer,
		},
	}
	return api
}
