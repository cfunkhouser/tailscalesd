package tailscalesd

import (
	"net/netip"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestTranslatePeerToDevice(t *testing.T) {
	want := Device{
		Addresses: []string{
			"100.2.3.4",
			"fd7a::1234",
		},
		API:        "localhost",
		Authorized: true,
		Hostname:   "somethingclever",
		ID:         "id",
		OS:         "beos",
		Tags: []string{
			"tag:foo",
			"tag:bar",
		},
	}
	var got Device
	translatePeerToDevice(&interestingPeerStatusSubset{
		ID:       "id",
		HostName: "somethingclever",
		DNSName:  "this is currently ignored",
		OS:       "beos",
		TailscaleIPs: []netip.Addr{
			netip.MustParseAddr("100.2.3.4"),
			netip.MustParseAddr("fd7a::1234"),
		},
		Tags: []string{
			"tag:foo",
			"tag:bar",
		},
	}, &got)
	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("translatePeerToDevice: mismatch (-got, +want):\n%v", diff)
	}
}
