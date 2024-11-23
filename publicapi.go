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
	"golang.org/x/oauth2/clientcredentials"
	"tailscale.com/client/tailscale"
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

func (a *publicAPIDiscoverer) Devices(ctx context.Context, excludeOffline bool) ([]Device, error) {
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

	var devices []Device
	for _, device := range d.Devices {
		if excludeOffline && !device.Online {
			continue
		}

		device.API = a.apiBase
		device.Tailnet = a.tailnet
		devices = append(devices, device)
	}
	return devices, nil
}

type OAuthPublicAPIDiscoverer struct {
	apiBase      string
	clientId     string
	clientSecret string
}

func (a *OAuthPublicAPIDiscoverer) Devices(ctx context.Context, excludeOffline bool) ([]Device, error) {
	tailscale.I_Acknowledge_This_API_Is_Unstable = true // needed in order to use API clients.

	start := time.Now()
	lv := prometheus.Labels{
		"api":  "public",
		"host": a.apiBase,
	}
	defer func() {
		apiRequestLatencyHistogram.With(lv).Observe(float64(time.Since(start).Milliseconds()))
	}()

	client := tailscale.NewClient("-", nil)
	client.BaseURL = "https://" + a.apiBase

	credentials := clientcredentials.Config{
		ClientID:     a.clientId,
		ClientSecret: a.clientSecret,
		TokenURL:     client.BaseURL + "/api/v2/oauth/token",
		Scopes:       []string{"device"},
	}

	client.HTTPClient = credentials.Client(ctx)

	tailnet := client.Tailnet()

	apiDevices, err := client.Devices(ctx, &tailscale.DeviceFieldsOpts{})
	if err != nil {
		apiRequestErrorCounter.With(lv).Inc()
		return nil, err
	}

	devices := make([]Device, len(apiDevices))

	for _, device := range apiDevices {
		if excludeOffline && device.LastSeen != "" {
			continue
		}
		devices = append(devices, Device{
			Addresses:     device.Addresses,
			API:           a.apiBase,
			Authorized:    device.Authorized,
			ClientVersion: device.ClientVersion,
			Hostname:      device.Hostname,
			ID:            device.DeviceID,
			Name:          device.Name,
			OS:            device.OS,
			Tailnet:       tailnet,
			Tags:          device.Tags,
		})
	}
	return devices, nil
}

type PublicAPIOption func(*publicAPIDiscoverer)
type OAuthAPIOption func(*OAuthPublicAPIDiscoverer)

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

// The OAuthAPI Discoverer polls the public Tailscale API for hosts in the tailnet.
func OAuthAPI(clientID string, clientSecret string, opts ...OAuthAPIOption) Discoverer {
	api := &OAuthPublicAPIDiscoverer{
		apiBase:      PublicAPIHost,
		clientId:     clientID,
		clientSecret: clientSecret,
	}
	for _, opt := range opts {
		opt(api)
	}

	return api
}
