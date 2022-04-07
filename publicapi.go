package tailscalesd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type deviceAPIResponse struct {
	Devices []Device `json:"devices"`
}

type publicAPIDiscoverer struct {
	client  *http.Client
	apiBase string
	tailnet string
	token   string
}

var errFailedAPIRequest = errors.New("failed API request")

func (a *publicAPIDiscoverer) Devices(ctx context.Context) ([]Device, error) {
	start := time.Now()
	lv := prometheus.Labels{
		"api":  "public",
		"host": a.apiBase,
	}
	defer func() {
		apiRequestLatencyHistogram.With(lv).Observe(float64(time.Since(start).Milliseconds()))
	}()

	url := fmt.Sprintf("https://%v@%v/api/v2/tailnet/%v/devices", a.token, a.apiBase, a.tailnet)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	apiRequestCounter.With(prometheus.Labels{
		"api":  "public",
		"host": a.apiBase,
	}).Inc()
	resp, err := a.client.Do(req)
	if err != nil {
		apiRequestErrorCounter.With(lv).Inc()
		return nil, err
	}
	if (resp.StatusCode / 100) != 2 {
		apiRequestErrorCounter.With(lv).Inc()
		return nil, fmt.Errorf("%w: %v", errFailedAPIRequest, resp.Status)
	}
	defer resp.Body.Close()
	var d deviceAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		apiPayloadErrorCounter.With(lv).Inc()
		return nil, fmt.Errorf("%w: bad payload from API: %v", errFailedAPIRequest, err)
	}
	tailnetDevicesFoundCounter.With(prometheus.Labels{"tailnet": a.tailnet}).Inc()
	for i := range d.Devices {
		d.Devices[i].API = a.apiBase
		d.Devices[i].Tailnet = a.tailnet
	}
	return d.Devices, nil
}

type PublicAPIOption func(*publicAPIDiscoverer)

// WithAPIHost sets the API base against which the PublicAPI Discoverers will
// attempt discovery. If not used, defaults to PublicAPIHost.
func WithAPIHost(host string) PublicAPIOption {
	return func(api *publicAPIDiscoverer) {
		api.apiBase = host
	}
}

// WithHTTPClient is a PublicAPIOption which allows callers to provide a HTTP
// client to PublicAPI instances. If not used, the defaultHTTPClient is used.
func WithHTTPClient(client *http.Client) PublicAPIOption {
	return func(api *publicAPIDiscoverer) {
		api.client = client
	}
}

// PublicAPIHost host for Tailscale.
const PublicAPIHost = "api.tailscale.com"

// defaultHTTPClient is shared across PublicAPI Discoverer instances by default.
// It strives to have sane enough defaults to not shoot users in the foot.
var defaultHTTPClient = &http.Client{
	Timeout: time.Second * 10,
	Transport: &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 5 * time.Second,
	},
}

// PublicAPI Discoverer polls the public Tailscale API for hosts in the tailnet.
func PublicAPI(tailnet, token string, opts ...PublicAPIOption) Discoverer {
	api := &publicAPIDiscoverer{
		apiBase: PublicAPIHost,
		tailnet: tailnet,
		token:   token,
	}
	for _, opt := range opts {
		opt(api)
	}
	if api.client == nil {
		api.client = defaultHTTPClient
	}
	return api
}
