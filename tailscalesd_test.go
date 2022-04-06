package tailscalesd

import (
	"context"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestMain(m *testing.M) {
	// No log output during test runs.
	log.SetOutput(ioutil.Discard)
	os.Exit(m.Run())
}

func TestExcludeEmptyMapEntries(t *testing.T) {
	for tn, tc := range map[string]struct {
		in   map[string]string
		want map[string]string
	}{
		"nil": {},
		"no empties": {
			in: map[string]string{
				"one fish": "two fish",
				"red fish": "blue fish",
			},
			want: map[string]string{
				"one fish": "two fish",
				"red fish": "blue fish",
			},
		},
		"empty key": {
			in: map[string]string{
				"":         "two fish",
				"red fish": "blue fish",
			},
			want: map[string]string{
				"red fish": "blue fish",
			},
		},
		"empty value": {
			in: map[string]string{
				"one fish": "two fish",
				"red fish": "",
			},
			want: map[string]string{
				"one fish": "two fish",
			},
		},
	} {
		t.Run(tn, func(t *testing.T) {
			got := excludeEmptyMapEntries(tc.in)
			if diff := cmp.Diff(got, tc.want); diff != "" {
				t.Errorf("excludeEmptyMapEntries: mismatch (-got, +want):\n%v", diff)
			}
		})
	}
}

func TestFilterIPv6Addresses(t *testing.T) {
	for tn, tc := range map[string]struct {
		descriptor TargetDescriptor
		want       TargetDescriptor
	}{
		"zero": {},
		"leaves ipv4 addresses alone": {
			descriptor: TargetDescriptor{
				Targets: []string{"100.2.3.4", "100.5.6.7"},
			},
			want: TargetDescriptor{
				Targets: []string{"100.2.3.4", "100.5.6.7"},
			},
		},
		"leaves ipv4 addresses alone while removing ipv6 addresses": {
			descriptor: TargetDescriptor{
				Targets: []string{"100.2.3.4", "100.5.6.7", "fd7a::1234", "fd7a::5678"},
			},
			want: TargetDescriptor{
				Targets: []string{"100.2.3.4", "100.5.6.7"},
			},
		},
		"leaves garbage alone without panicking or whatever": {
			descriptor: TargetDescriptor{
				Targets: []string{"100.2.3.4", "GARBAGE"},
			},
			want: TargetDescriptor{
				Targets: []string{"100.2.3.4", "GARBAGE"},
			},
		},
		"leaves garbage alone without panicking while removing ipv6 addresses": {
			descriptor: TargetDescriptor{
				Targets: []string{"100.2.3.4", "GARBAGE", "fd7a::1234"},
			},
			want: TargetDescriptor{
				Targets: []string{"100.2.3.4", "GARBAGE"},
			},
		},
	} {
		t.Run(tn, func(t *testing.T) {
			got := filterIPv6Addresses(tc.descriptor)
			if diff := cmp.Diff(got, tc.want); diff != "" {
				t.Errorf("filterIPv6Addresses: mismatch (-got, +want):\n%v", diff)
			}
		})
	}
}

func TestTranslate(t *testing.T) {
	for tn, tc := range map[string]struct {
		devices []Device
		filters []filter
		want    []TargetDescriptor
	}{
		"zero": {},
		"single device without tags expands to single descriptor": {
			devices: []Device{
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
				},
			},
			want: []TargetDescriptor{
				{
					Targets: []string{"100.2.3.4", "fd7a::1234"},
					Labels: map[string]string{
						"__meta_tailscale_api":                   "foo.example.com",
						"__meta_tailscale_device_authorized":     "false",
						"__meta_tailscale_device_client_version": "420.69",
						"__meta_tailscale_device_hostname":       "somethingclever",
						"__meta_tailscale_device_id":             "id",
						"__meta_tailscale_device_name":           "somethingclever",
						"__meta_tailscale_device_os":             "beos",
						"__meta_tailscale_tailnet":               "example@gmail.com",
					},
				},
			},
		},
		"single device with two tags expands to two descriptors": {
			devices: []Device{
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
			},
			want: []TargetDescriptor{
				{
					Targets: []string{"100.2.3.4", "fd7a::1234"},
					Labels: map[string]string{
						"__meta_tailscale_api":                   "foo.example.com",
						"__meta_tailscale_device_authorized":     "false",
						"__meta_tailscale_device_client_version": "420.69",
						"__meta_tailscale_device_hostname":       "somethingclever",
						"__meta_tailscale_device_id":             "id",
						"__meta_tailscale_device_name":           "somethingclever",
						"__meta_tailscale_device_os":             "beos",
						"__meta_tailscale_device_tag":            "tag:foo",
						"__meta_tailscale_tailnet":               "example@gmail.com",
					},
				},
				{
					Targets: []string{"100.2.3.4", "fd7a::1234"},
					Labels: map[string]string{
						"__meta_tailscale_api":                   "foo.example.com",
						"__meta_tailscale_device_authorized":     "false",
						"__meta_tailscale_device_client_version": "420.69",
						"__meta_tailscale_device_hostname":       "somethingclever",
						"__meta_tailscale_device_id":             "id",
						"__meta_tailscale_device_name":           "somethingclever",
						"__meta_tailscale_device_os":             "beos",
						"__meta_tailscale_device_tag":            "tag:bar",
						"__meta_tailscale_tailnet":               "example@gmail.com",
					},
				},
			},
		},
		"filters apply to all descriptors expanded from device": {
			devices: []Device{
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
			},
			filters: []filter{
				func(in TargetDescriptor) TargetDescriptor {
					in.Labels["test_label"] = "IT WORKED"
					return in
				},
			},
			want: []TargetDescriptor{
				{
					Targets: []string{"100.2.3.4", "fd7a::1234"},
					Labels: map[string]string{
						"__meta_tailscale_api":                   "foo.example.com",
						"__meta_tailscale_device_authorized":     "false",
						"__meta_tailscale_device_client_version": "420.69",
						"__meta_tailscale_device_hostname":       "somethingclever",
						"__meta_tailscale_device_id":             "id",
						"__meta_tailscale_device_name":           "somethingclever",
						"__meta_tailscale_device_os":             "beos",
						"__meta_tailscale_device_tag":            "tag:foo",
						"__meta_tailscale_tailnet":               "example@gmail.com",
						"test_label":                             "IT WORKED",
					},
				},
				{
					Targets: []string{"100.2.3.4", "fd7a::1234"},
					Labels: map[string]string{
						"__meta_tailscale_api":                   "foo.example.com",
						"__meta_tailscale_device_authorized":     "false",
						"__meta_tailscale_device_client_version": "420.69",
						"__meta_tailscale_device_hostname":       "somethingclever",
						"__meta_tailscale_device_id":             "id",
						"__meta_tailscale_device_name":           "somethingclever",
						"__meta_tailscale_device_os":             "beos",
						"__meta_tailscale_device_tag":            "tag:bar",
						"__meta_tailscale_tailnet":               "example@gmail.com",
						"test_label":                             "IT WORKED",
					},
				},
			},
		},
	} {
		t.Run(tn, func(t *testing.T) {
			got := translate(tc.devices, tc.filters...)
			if diff := cmp.Diff(got, tc.want); diff != "" {
				t.Errorf("translate: mismatch (-got, +want):\n%v", diff)
			}
		})
	}
}

type testDiscoverer struct {
	Called     int
	discovered []Device
	err        error
}

func (t *testDiscoverer) Devices(_ context.Context) ([]Device, error) {
	t.Called++
	return t.discovered, t.err
}

type httpWant struct {
	code        int
	contentType string
	body        string // string for prettier cmp diff output.
}

func TestDiscoveryHandler(t *testing.T) {
	for tn, tc := range map[string]struct {
		discoverer Discoverer
		want       httpWant
	}{
		"nil": {
			want: httpWant{
				code: http.StatusInternalServerError,
				body: "Attempted to serve with an improperly initialized handler.",
			},
		},
		"unspecified API error": {
			discoverer: &testDiscoverer{
				err: errors.New("this is a test error"),
			},
			want: httpWant{
				code: http.StatusInternalServerError,
				body: "Failed to discover Tailscale devices: this is a test error",
			},
		},
		"stale results are still served": {
			discoverer: &testDiscoverer{
				discovered: []Device{
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
				},
				err: errStaleResults,
			},
			want: httpWant{
				code:        http.StatusOK,
				contentType: "application/json; charset=utf-8",
				body:        `[{"targets":["100.2.3.4"],"labels":{"__meta_tailscale_api":"foo.example.com","__meta_tailscale_device_authorized":"false","__meta_tailscale_device_client_version":"420.69","__meta_tailscale_device_hostname":"somethingclever","__meta_tailscale_device_id":"id","__meta_tailscale_device_name":"somethingclever","__meta_tailscale_device_os":"beos","__meta_tailscale_device_tag":"tag:foo","__meta_tailscale_tailnet":"example@gmail.com"}},{"targets":["100.2.3.4"],"labels":{"__meta_tailscale_api":"foo.example.com","__meta_tailscale_device_authorized":"false","__meta_tailscale_device_client_version":"420.69","__meta_tailscale_device_hostname":"somethingclever","__meta_tailscale_device_id":"id","__meta_tailscale_device_name":"somethingclever","__meta_tailscale_device_os":"beos","__meta_tailscale_device_tag":"tag:bar","__meta_tailscale_tailnet":"example@gmail.com"}}]` + "\n",
			},
		},
		"results with no errors are served": {
			discoverer: &testDiscoverer{
				discovered: []Device{
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
				},
			},
			want: httpWant{
				code:        http.StatusOK,
				contentType: "application/json; charset=utf-8",
				body:        `[{"targets":["100.2.3.4"],"labels":{"__meta_tailscale_api":"foo.example.com","__meta_tailscale_device_authorized":"false","__meta_tailscale_device_client_version":"420.69","__meta_tailscale_device_hostname":"somethingclever","__meta_tailscale_device_id":"id","__meta_tailscale_device_name":"somethingclever","__meta_tailscale_device_os":"beos","__meta_tailscale_device_tag":"tag:foo","__meta_tailscale_tailnet":"example@gmail.com"}},{"targets":["100.2.3.4"],"labels":{"__meta_tailscale_api":"foo.example.com","__meta_tailscale_device_authorized":"false","__meta_tailscale_device_client_version":"420.69","__meta_tailscale_device_hostname":"somethingclever","__meta_tailscale_device_id":"id","__meta_tailscale_device_name":"somethingclever","__meta_tailscale_device_os":"beos","__meta_tailscale_device_tag":"tag:bar","__meta_tailscale_tailnet":"example@gmail.com"}}]` + "\n",
			},
		},
	} {
		t.Run(tn, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			w := httptest.NewRecorder()

			Export(tc.discoverer).ServeHTTP(w, r)

			if w.Code != tc.want.code {
				t.Errorf("discoveryHandler: status code mismatch: got: %v want: %v", w.Code, tc.want.code)
			}
			if diff := cmp.Diff(w.Body.String(), tc.want.body); diff != "" {
				t.Errorf("discoveryHandler: content mismatch (-got, +want):\n%v", diff)
			}
		})
	}
}
