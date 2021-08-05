// Package tailscalesd provides Prometheus Service Discovery for Tailscale.
package tailscalesd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/cfunkhouser/tailscalesd/internal/tailscale"
)

// TargetDescriptor as Prometheus expects it. For more details, see
// https://prometheus.io/docs/prometheus/latest/http_sd/.
type TargetDescriptor struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels,omitempty"`
}

// v2API describes subset of the Tailscale API needed for discovering things.
type v2API interface {
	Devices(context.Context) ([]tailscale.Device, error)
}

const (
	labelMetaDeviceAuthorized    = "__meta_tailscale_device_authorized"
	labelMetaDeviceClientVersion = "__meta_tailscale_device_client_version"
	labelMetaDeviceHostname      = "__meta_tailscale_device_hostname"
	labelMetaDeviceID            = "__meta_tailscale_device_id"
	labelMetaDeviceIsExternal    = "__meta_tailscale_device_is_external"
	labelMetaDeviceMachineKey    = "__meta_tailscale_device_machine_key"
	labelMetaDeviceName          = "__meta_tailscale_device_name"
	labelMetaDeviceNodeKey       = "__meta_tailscale_device_node_key"
	labelMetaDeviceOS            = "__meta_tailscale_device_os"
	labelMetaDeviceUser          = "__meta_tailscale_device_user"
)

// filterEmpty removes entries in a map which have either an empty key or empty
// value.
func filterEmpty(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	filtered := make(map[string]string)
	for k, v := range in {
		if k == "" || v == "" {
			continue
		}
		filtered[k] = v
	}
	return filtered
}

type filter func(TargetDescriptor) TargetDescriptor

func filterIPv6Addresses(td TargetDescriptor) TargetDescriptor {
	var targets []string
	for _, target := range td.Targets {
		ip := net.ParseIP(target)
		if ip == nil {
			// target is not a valid IP address of any version.
			continue
		}
		if ipv4 := ip.To4(); ipv4 != nil {
			targets = append(targets, ipv4.String())
		}
	}
	return TargetDescriptor{
		Targets: targets,
		Labels:  td.Labels,
	}
}

func filterEmptyLabels(td TargetDescriptor) TargetDescriptor {
	return TargetDescriptor{
		Targets: td.Targets,
		Labels:  filterEmpty(td.Labels),
	}
}

// translate Devices to Prometheus TargetDescriptor, filtering empty labels.
func translate(devices []tailscale.Device, filters ...filter) (found []TargetDescriptor) {
	for _, d := range devices {
		target := TargetDescriptor{
			Targets: d.Addresses,
			Labels: map[string]string{
				labelMetaDeviceAuthorized:    fmt.Sprint(d.Authorized),
				labelMetaDeviceClientVersion: d.ClientVersion,
				labelMetaDeviceHostname:      d.Hostname,
				labelMetaDeviceID:            d.ID,
				labelMetaDeviceIsExternal:    fmt.Sprint(d.IsExternal),
				labelMetaDeviceMachineKey:    d.MachineKey,
				labelMetaDeviceName:          d.Name,
				labelMetaDeviceNodeKey:       d.NodeKey,
				labelMetaDeviceOS:            d.OS,
				labelMetaDeviceUser:          d.User,
			},
		}
		for _, filter := range filters {
			target = filter(target)
		}
		found = append(found, target)
	}
	return
}

type discoverer struct {
	ts v2API
}

// DiscoverDevices in a tailnet.
func (d *discoverer) DiscoverDevices(ctx context.Context) ([]TargetDescriptor, error) {
	devices, err := d.ts.Devices(ctx)
	if err != nil {
		return nil, err
	}
	return translate(devices, filterEmptyLabels, filterIPv6Addresses), nil
}

var ErrStaleResults = errors.New("potentially stale results")

type rateLimitingDiscoverer struct {
	sync.RWMutex
	discoverer Discoverer
	freq       time.Duration

	// protected
	lastDevices []TargetDescriptor
	earliest    time.Time
}

func (d *rateLimitingDiscoverer) refreshDevices(ctx context.Context) ([]TargetDescriptor, error) {
	log.Printf("Refreshing Devices")
	devices, err := d.discoverer.DiscoverDevices(ctx)
	if err != nil {
		return devices, err
	}

	d.Lock()
	defer d.Unlock()

	d.lastDevices = devices
	d.earliest = time.Now().Add(d.freq)
	log.Printf("Device refresh successful. Next refresh no sooner than %v", d.earliest.Format(time.RFC3339))
	return devices, nil
}

func (d *rateLimitingDiscoverer) DiscoverDevices(ctx context.Context) ([]TargetDescriptor, error) {
	d.RLock()
	expired := time.Now().After(d.earliest)
	last := make([]TargetDescriptor, len(d.lastDevices))
	_ = copy(last, d.lastDevices)
	d.RUnlock()

	if expired {
		devices, err := d.refreshDevices(ctx)
		if err != nil {
			log.Printf("Failed refreshing: %v", err)
			return last, ErrStaleResults
		}
		return devices, nil
	}
	return last, nil
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

type Option func(Discoverer) Discoverer

func WithRateLimit(freq time.Duration) Option {
	return func(d Discoverer) Discoverer {
		return &rateLimitingDiscoverer{
			discoverer: d,
			freq:       freq,
		}
	}
}

// Discoverer of things in a tailnet.
type Discoverer interface {
	DiscoverDevices(ctx context.Context) ([]TargetDescriptor, error)
}

func New(tailnet, token string, opts ...Option) Discoverer {
	var d Discoverer = &discoverer{
		ts: &tailscale.API{
			Client:  defaultHTTPClient(),
			APIBase: tailscale.ProductionAPI,
			Tailnet: tailnet,
			Token:   token,
		},
	}
	for _, opt := range opts {
		d = opt(d)
	}
	return d
}

// discoveryHandler is a http.Handler that exposes the SD payload. It caches the
// last valid payload for a fixed period of time to prevent hammering Tailscale's
// API.
type discoveryHandler struct {
	d Discoverer
}

func (h *discoveryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	targets, err := h.d.DiscoverDevices(r.Context())
	if err != nil {
		if err != ErrStaleResults {
			w.WriteHeader(http.StatusInternalServerError)
			log.Printf("Failed to discover Tailscale devices: %v", err)
			fmt.Fprintf(w, "Failed to discover Tailscale devices: %v", err)
			return
		}
		log.Print("Serving potentially stale results")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(targets); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("Failed encoding targets to JSON: %v", err)
		fmt.Fprintf(w, "Failed encoding targets to JSON: %v", err)
		return
	}

	w.Header().Add("Content-Type", "application/json")
	if _, err := io.Copy(w, &buf); err != nil {
		// The transaction with the client is already started, so there's nothing
		// graceful to do here. Log any errors for troubleshooting later.
		log.Printf("Failed sending JSON payload to the client: %v", err)
	}
}

// Export the Discoverer as a http.Handler.
func Export(d Discoverer, pollFrequency time.Duration) http.Handler {
	return &discoveryHandler{d}
}
