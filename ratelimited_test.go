package tailscalesd

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

var devicesForRatelimitedTest = []Device{
	{
		Addresses: []string{
			"100.2.3.4",
			"fd7a::1234",
		},
		API:           "foo.example.com",
		ClientVersion: "420.69",
		Hostname:      "somethingclever",
		ID:            "id",
		Name:          "somethingclever",
		OS:            "beos",
		Tailnet:       "example@gmail.com",
		Tags: []string{
			"tag:foo",
			"tag:bar",
		},
	},
}

func discovererForTest(tb testing.TB) *testDiscoverer {
	tb.Helper()
	return &testDiscoverer{
		discovered: devicesForRatelimitedTest,
	}
}

type rateLimitedDiscovererTestWant struct {
	called  int
	err     error
	devices []Device
}

// TestRateLimitedDiscoverer tests the RateLimitedDiscoverer's behavior. It uses
// time math, but limits the chances of flakiness by using increments of 30
// hours. If this test takes more than 30 hours to run, it may fail erroneously.
// If this test takes more than 30 hours to run, you need a new computer.
func TestRateLimitedDiscoverer(t *testing.T) {
	for tn, tc := range map[string]struct {
		discoverer *RateLimitedDiscoverer
		wrapped    *testDiscoverer
		want       rateLimitedDiscovererTestWant
	}{
		"rate limited discoverer which has never been used calls Discover": {
			discoverer: &RateLimitedDiscoverer{},
			wrapped:    discovererForTest(t),
			want: rateLimitedDiscovererTestWant{
				called:  1,
				devices: devicesForRatelimitedTest,
			},
		},
		"rate limited discoverer which is expired calls Discover": {
			discoverer: &RateLimitedDiscoverer{
				earliest: time.Now().Add(-30 * time.Hour),
			},
			wrapped: discovererForTest(t),
			want: rateLimitedDiscovererTestWant{
				called:  1,
				devices: devicesForRatelimitedTest,
			},
		},
		"rate limited discoverer which is not expired returns cached results": {
			discoverer: &RateLimitedDiscoverer{
				earliest: time.Now().Add(30 * time.Hour),
				last: []Device{
					{ID: "ratelimittest"},
				},
			},
			wrapped: discovererForTest(t),
			want: rateLimitedDiscovererTestWant{
				devices: []Device{
					{ID: "ratelimittest"},
				},
			},
		},
		"rate limited discoverer which is expired returns cached results on error": {
			discoverer: &RateLimitedDiscoverer{
				earliest: time.Now().Add(30 * time.Hour),
				last: []Device{
					{ID: "ratelimittest"},
				},
			},
			wrapped: &testDiscoverer{
				err: errors.New("this is a test error"), //nolint:err113
			},
			want: rateLimitedDiscovererTestWant{
				devices: []Device{
					{ID: "ratelimittest"},
				},
			},
		},
	} {
		t.Run(tn, func(t *testing.T) {
			tc.discoverer.Wrap = tc.wrapped
			got, err := tc.discoverer.Devices(context.TODO())
			if !errors.Is(err, tc.want.err) {
				t.Errorf("RateLimitedDiscoverer: unexpected error: %v", err)
			}
			if got, want := tc.wrapped.Called, tc.want.called; got != want {
				t.Errorf("RateLimitedDiscoverer: mismatched Discover call count: got: %d want: %d", got, want)
			}
			if diff := cmp.Diff(got, tc.want.devices); diff != "" {
				t.Errorf("RateLimitedDiscoverer: mismatch (-got, +want):\n%v", diff)
			}
		})
	}
}
