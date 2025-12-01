package tailscalesd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func apiBaseForTest(tb testing.TB, surl string) string {
	tb.Helper()
	u, err := url.Parse(surl)
	if err != nil {
		tb.Fatal(err)
	}
	return u.Host
}

func TestPublicAPIDiscovererDevices(t *testing.T) {
	var wantPath = "/api/v2/tailnet/testTailnet/devices"
	for tn, tc := range map[string]struct {
		responder func(w http.ResponseWriter)
		wantErr   error
		want      []Device
	}{
		"returns failed request error when the server responds unsuccessfully": {
			responder: func(w http.ResponseWriter) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantErr: errFailedAPIRequest,
		},
		"returns failed request error when the server responds with bad payload": {
			responder: func(w http.ResponseWriter) {
				w.Header().Set("Content-Type", "text/plain")
				_, _ = fmt.Fprintln(w, "This is decidedly not JSON.")
			},
			wantErr: errFailedAPIRequest,
		},
		"returns devices when the server responds with valid JSON": {
			responder: func(w http.ResponseWriter) {
				w.Header().Set("Content-Type", "application/json; encoding=utf-8")
				_, _ = w.Write([]byte(`{"devices": [{"hostname":"testhostname","os":"beos"}]}`))
			},
			want: []Device{
				{
					Hostname: "testhostname",
					OS:       "beos",
					Tailnet:  "testTailnet",
				},
			},
		},
	} {
		t.Run(tn, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got, want := r.URL.Path, wantPath; got != want {
					t.Errorf("Devices: request URL path mismatch: got: %q want: %q", got, want)
				}
				tc.responder(w)
			}))
			defer server.Close()

			d := PublicAPI("testTailnet", "testToken", WithHTTPClient(server.Client()), WithAPIHost(apiBaseForTest(t, server.URL)))
			got, err := d.Devices(context.TODO())
			if got, want := err, tc.wantErr; !errors.Is(got, want) {
				t.Errorf("Devices: error mismatch: got: %q want: %q", got, want)
			}
			// Ignore the API field, which will be set to the arbitrary test
			// server's host:port.
			if diff := cmp.Diff(got, tc.want, cmpopts.IgnoreFields(Device{}, "API")); diff != "" {
				t.Errorf("PublicAPI: mismatch (-got, +want):\n%v", diff)
			}
		})
	}
}

func TestWithAPIHost(t *testing.T) {
	want := "test.example.com"
	var got publicAPIDiscoverer
	WithAPIHost(want)(&got)
	if got.apiBase != want {
		t.Errorf("WithAPIHost: apiBase mismatch: got: %q want: %q", got.apiBase, want)
	}
}

func TestWithHTTPClient(t *testing.T) {
	want := &http.Client{}
	var got publicAPIDiscoverer
	WithHTTPClient(want)(&got)
	if got.client != want {
		t.Errorf("WithHTTPClient: client mismatch: got: %+v want: %+v", got.client, want)
	}
}

type publicAPIOptTester struct {
	called int
}

func (t *publicAPIOptTester) Opt() PublicAPIOption {
	return func(_ *publicAPIDiscoverer) {
		t.called++
	}
}

func publicAPIDiscovererComparer(l, r *publicAPIDiscoverer) bool {
	return l.client == r.client &&
		l.apiBase == r.apiBase &&
		l.tailnet == r.tailnet &&
		l.token == r.token
}

func TestPublicAPISetsDefaults(t *testing.T) {
	got, ok := PublicAPI("testTailnet", "testToken").(*publicAPIDiscoverer)
	if !ok {
		t.Fatalf("PublicAPI: type mismatch: the Discoverer returned by PublicAPI() was not a *publicAPIDiscoverer")
	}
	want := &publicAPIDiscoverer{
		client:  defaultHTTPClient,
		apiBase: PublicAPIHost,
		tailnet: "testTailnet",
		token:   "testToken",
	}
	if diff := cmp.Diff(got, want, cmp.Comparer(publicAPIDiscovererComparer)); diff != "" {
		t.Errorf("PublicAPI: mismatch (-got, +want):\n%v", diff)
	}
}

func TestPublicAPIInvokesAllOptionsExactlyOnce(t *testing.T) {
	optTesters := make([]publicAPIOptTester, 25)

	opts := make([]PublicAPIOption, len(optTesters))
	for i := range optTesters {
		opts[i] = optTesters[i].Opt()
	}

	_ = PublicAPI("ignored", "ignored", opts...)

	for i := range optTesters {
		if got, want := optTesters[i].called, 1; got != want {
			t.Errorf("PublicAPI: option call mismatch: got: %d want: %d", got, want)
		}
	}
}
