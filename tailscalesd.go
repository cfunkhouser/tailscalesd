// Package tailscalesd provides Prometheus Service Discovery for Tailscale using
// a naive, bespoke Tailscale API client supporting both the public v2 and local
// APIs. It has only the functionality needed for tailscalesd. You should not
// be tempted to use it for anything else.
package tailscalesd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
)

const (
	// LabelMetaAPI is the host which provided the details about this device.
	// Will be "localhost" for the local API.
	LabelMetaAPI = "__meta_tailscale_api"

	// LabelMetaDeviceAuthorized is whether the target is currently authorized
	// on the Tailnet. Will always be true when using the local API.
	LabelMetaDeviceAuthorized = "__meta_tailscale_device_authorized"

	// LabelMetaDeviceClientVersion is the Tailscale client version in use on
	// target. Not reported when using the local API.
	LabelMetaDeviceClientVersion = "__meta_tailscale_device_client_version"

	// LabelMetaDeviceHostname is the short hostname of the device.
	LabelMetaDeviceHostname = "__meta_tailscale_device_hostname"

	// LabelMetaDeviceID is the target's unique ID within Tailscale, as reported
	// by the API. The public API reports this as a large integer. The local API
	// reports a base64 string.
	// string.
	LabelMetaDeviceID = "__meta_tailscale_device_id"

	// LabelMetaDeviceName is the name of the device as reported by the API. Not
	// reported when using the local API.
	LabelMetaDeviceName = "__meta_tailscale_device_name"

	// LabelMetaDeviceOS is the OS of the target.
	LabelMetaDeviceOS = "__meta_tailscale_device_os"

	// LabelMetaDeviceTag is a Tailscale ACL tag applied to the target.
	LabelMetaDeviceTag = "__meta_tailscale_device_tag"

	// LabelMetaTailnet is the name of the Tailnet from which this target
	// information was retrieved. Not reported when using the local API.
	LabelMetaTailnet = "__meta_tailscale_tailnet"
)

// Device in a Tailnet, as reported by one of the various Tailscale APIs.
type Device struct {
	Addresses     []string `json:"addresses"`
	API           string   `json:"api"`
	Authorized    bool     `json:"authorized"`
	ClientVersion string   `json:"clientVersion,omitempty"`
	Hostname      string   `json:"hostname"`
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	OS            string   `json:"os"`
	Tailnet       string   `json:"tailnet"`
	Tags          []string `json:"tags"`
}

// Discoverer of things exposed by the various Tailscale APIs.
type Discoverer interface {
	// Devices reported by the Tailscale public API as belonging to the
	// configured tailnet.
	Devices(context.Context) ([]Device, error)
}

// TargetDescriptor as Prometheus expects it. For more details, see
// https://prometheus.io/docs/prometheus/latest/http_sd/.
type TargetDescriptor struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels,omitempty"`
}

// TargetFilter manipulates TargetDescriptors before being served.
type TargetFilter func(TargetDescriptor) TargetDescriptor

// FilterIPv6Addresses from TargetDescriptors. Results in only IPv4 targets.
func FilterIPv6Addresses(td TargetDescriptor) TargetDescriptor {
	var targets []string
	for _, target := range td.Targets {
		ip := net.ParseIP(target)
		if ip == nil {
			// target is not a valid IP address of any version, but this filter
			// is explicitly for IPv6 addresses, so we leave the garbage in
			// place.
			targets = append(targets, target)
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

// excludeEmptyMapEntries removes entries in a map which have either an empty
// key or empty value.
func excludeEmptyMapEntries(in map[string]string) map[string]string {
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

func filterEmptyLabels(td TargetDescriptor) TargetDescriptor {
	return TargetDescriptor{
		Targets: td.Targets,
		Labels:  excludeEmptyMapEntries(td.Labels),
	}
}

// translate Devices to Prometheus TargetDescriptor, filtering empty labels.
func translate(devices []Device, filters ...TargetFilter) (found []TargetDescriptor) {
	for _, d := range devices {
		target := TargetDescriptor{
			Targets: d.Addresses,
			// All labels added here, except for tags.
			Labels: map[string]string{
				LabelMetaAPI:                 d.API,
				LabelMetaDeviceAuthorized:    fmt.Sprint(d.Authorized),
				LabelMetaDeviceClientVersion: d.ClientVersion,
				LabelMetaDeviceHostname:      d.Hostname,
				LabelMetaDeviceID:            d.ID,
				LabelMetaDeviceName:          d.Name,
				LabelMetaDeviceOS:            d.OS,
				LabelMetaTailnet:             d.Tailnet,
			},
		}
		for _, filter := range filters {
			target = filter(target)
		}
		if l := len(d.Tags); l == 0 {
			found = append(found, target)
			continue
		}
		for _, t := range d.Tags {
			lt := target
			lt.Labels = make(map[string]string)
			for k, v := range target.Labels {
				lt.Labels[k] = v
			}
			lt.Labels[LabelMetaDeviceTag] = t
			found = append(found, lt)
		}
	}
	return
}

type discoveryHandler struct {
	d       Discoverer
	filters []TargetFilter
}

func (h discoveryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.d == nil {
		const msg = "Attempted to serve with an improperly initialized handler"
		slog.Error(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	devices, err := h.d.Devices(r.Context())
	if err != nil {
		if !errors.Is(err, errStaleResults) {
			const msg = "Failed to discover Tailscale devices"
			slog.Error(msg, "error", err)
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}

		// TODO(cfunkhouser): Investigate whether Prometheus respects cache
		// control headers, and implement accordingly here.
		slog.Info("Serving potentially stale results")
	}
	targets := translate(devices, h.filters...)

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(targets); err != nil {
		const msg = "Failed to encode targets as JSON"
		slog.Error(msg, "error", err)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	if _, err := io.Copy(w, &buf); err != nil {
		// The transaction with the client is already started, so there's
		// nothing graceful to do here. Log any errors for troubleshooting
		// later.
		slog.Debug("Failed sending JSON payload to the client", "error", err)
	}
}

// Empty labels must always be removed.
var defaultFilters = []TargetFilter{filterEmptyLabels}

// Export the Tailscale Discoverer for Service Discovery via HTTP, optionally
// applying filters to the discovery results.
func Export(d Discoverer, with ...TargetFilter) http.Handler {
	return discoveryHandler{
		d:       d,
		filters: append(defaultFilters, with...),
	}
}
