// Package tailscale contains types needed for both API implementations.
package tailscale

// PublicAPI host for Tailscale.
const PublicAPI = "api.tailscale.com"

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
}
