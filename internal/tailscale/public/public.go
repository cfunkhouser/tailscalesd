// Package public is a naive, bespoke Tailscale V2 API client. It has only the
// functionality needed for tailscalesd. You should not rely on its API for
// anything else.
package public

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/cfunkhouser/tailscalesd/internal/tailscale"
)

type deviceAPIResponse struct {
	Devices []tailscale.Device `json:"devices"`
}

// API client for the Tailscale public API.
type API struct {
	Client  *http.Client
	APIBase string
	Tailnet string
	Token   string
}

var ErrFailedRequest = errors.New("failed API call")

// Devices reported by the Tailscale public API as belonging to the configured
// tailnet.
func (a *API) Devices(ctx context.Context) ([]tailscale.Device, error) {
	url := fmt.Sprintf("https://%v@%v/api/v2/tailnet/%v/devices", a.Token, a.APIBase, a.Tailnet)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.Client.Do(req)
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
		d.Devices[i].API = a.APIBase
		d.Devices[i].Tailnet = a.Tailnet
	}
	return d.Devices, nil
}
