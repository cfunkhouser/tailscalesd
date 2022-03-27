// Package tailscale is a naive, bespoke Tailscale API client supporting both
// the public v2 and local APIs. It has only the functionality needed for
// tailscalesd. You should not rely on it for anything else.
package tailscalesd

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"
)

// PublicAPIHost host for Tailscale.
const PublicAPIHost = "api.tailscale.com"

type deviceAPIResponse struct {
	Devices []Device `json:"devices"`
}

type publicAPIClient struct {
	client  *http.Client
	apiBase string
	tailnet string
	token   string
}

func (a *publicAPIClient) Devices(ctx context.Context) ([]Device, error) {
	url := fmt.Sprintf("https://%v@%v/api/v2/tailnet/%v/devices", a.token, a.apiBase, a.tailnet)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	if (resp.StatusCode / 100) != 2 {
		return nil, fmt.Errorf("%w: %v", ErrFailedRequest, resp.Status)
	}
	defer resp.Body.Close()
	var d deviceAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return nil, err
	}
	for i := range d.Devices {
		d.Devices[i].API = a.apiBase
		d.Devices[i].Tailnet = a.tailnet
	}
	return d.Devices, nil
}

func defaultHTTPClient() *http.Client {
	return &http.Client{
		Timeout: time.Second * 10,
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout: 5 * time.Second,
			}).Dial,
			TLSHandshakeTimeout: 5 * time.Second,
		},
	}
}

type PublicAPIOption func(*publicAPIClient)

func WithAPIHost(host string) PublicAPIOption {
	return func(api *publicAPIClient) {
		api.apiBase = host
	}
}

func WithHTTPClient(client *http.Client) PublicAPIOption {
	return func(api *publicAPIClient) {
		api.client = client
	}
}

// PublicAPI client polls the public Tailscale API for hosts in the tailnet.
func PublicAPI(tailnet, token string, opts ...PublicAPIOption) Client {
	api := &publicAPIClient{
		apiBase: PublicAPIHost,
		tailnet: tailnet,
		token:   token,
	}
	for _, opt := range opts {
		opt(api)
	}
	if api.client == nil {
		api.client = defaultHTTPClient()
	}
	return api
}
