package tailscale

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Device in a Tailnet, as reported by the Tailscale API.
type Device struct {
	Addresses                 []string `json:"addresses"`
	Authorized                bool     `json:"authorized"`
	BlocksIncomingConnections bool     `json:"blocksIncomingConnections"`
	ClientVersion             string   `json:"clientVersion"`
	Created                   string   `json:"created"`
	Expires                   string   `json:"expires"`
	Hostname                  string   `json:"hostname"`
	ID                        string   `json:"id"`
	IsExternal                bool     `json:"isExternal"`
	KeyExpiryDisabled         bool     `json:"keyExpiryDisabled"`
	LastSeen                  string   `json:"lastSeen"`
	MachineKey                string   `json:"machineKey"`
	Name                      string   `json:"name"`
	NodeKey                   string   `json:"nodeKey"`
	OS                        string   `json:"os"`
	UpdateAvailable           bool     `json:"updateAvailable"`
	User                      string   `json:"user"`
}

type deviceAPIResponse struct {
	Devices []Device `json:"devices"`
}

const ProductionAPI = "api.tailscale.com"

type API struct {
	Client  *http.Client
	APIBase string
	Tailnet string
	Token   string
}

var ErrUnsuccessfulRequest = errors.New("unsuccessful API call")

func (c *API) Devices(ctx context.Context) ([]Device, error) {
	url := fmt.Sprintf("https://%v@%v/api/v2/tailnet/%v/devices", c.Token, c.APIBase, c.Tailnet)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	if (resp.StatusCode / 100) != 2 {
		return nil, fmt.Errorf("%w: %v", ErrUnsuccessfulRequest, resp.Status)
	}
	defer resp.Body.Close()
	var devices deviceAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&devices); err != nil {
		return nil, err
	}
	return devices.Devices, nil
}
