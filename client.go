// Package tailscale contains types needed for both API implementations.
package tailscalesd

import (
	"context"
	"errors"
)

var ErrFailedRequest = errors.New("failed localapi call")

// Client interface for the various Tailscale APIs.
type Client interface {
	// Devices reported by the Tailscale public API as belonging to the configured
	// tailnet.
	Devices(context.Context) ([]Device, error)
}

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
