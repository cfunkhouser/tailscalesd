package tailscalesd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/cfunkhouser/tailscalesd/tailscale"
)

type TargetDescriptor struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels,omitempty"`
}

type Discoverer struct {
	ts V2API
}

type V2API interface {
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

func filterEmpty(in map[string]string) map[string]string {
	r := make(map[string]string)
	for k, v := range in {
		if k == "" || v == "" {
			continue
		}
		r[k] = v
	}
	return r
}

func (d *Discoverer) Discover(ctx context.Context) ([]TargetDescriptor, error) {
	devices, err := d.ts.Devices(ctx)
	if err != nil {
		return nil, err
	}
	var found []TargetDescriptor
	for _, d := range devices {
		found = append(found, TargetDescriptor{
			Targets: d.Addresses,
			Labels: filterEmpty(map[string]string{
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
			}),
		})
	}
	return found, nil
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

func New(tailnet, token string) *Discoverer {
	return &Discoverer{
		ts: &tailscale.API{
			Client:  defaultHTTPClient(),
			APIBase: tailscale.ProductionAPI,
			Tailnet: tailnet,
			Token:   token,
		},
	}
}

type rateLimitingHandler struct {
	sync.RWMutex
	d    *Discoverer
	freq time.Duration

	// protected
	earliest time.Time
	last     []TargetDescriptor
}

func (h *rateLimitingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var targets []TargetDescriptor
	now := time.Now()
	h.RLock()
	should := h.earliest.Before(now)
	if !should {
		targets = h.last
	}
	h.RUnlock()

	if should {
		log.Printf("Fetching devices from Tailscale API")
		var err error
		targets, err = h.d.Discover(r.Context())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Printf("Failed to discover Tailscale devices: %v", err)
			fmt.Fprintf(w, "Failed to discover Tailscale devices: %v", err)
			return
		}
		h.Lock()
		h.last = targets
		h.earliest = now.Add(h.freq)
		h.Unlock()
	}

	w.Header().Add("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(targets)
}

func Export(d *Discoverer, maxFrequency time.Duration) http.Handler {
	return &rateLimitingHandler{
		d:    d,
		freq: maxFrequency,
	}
}
