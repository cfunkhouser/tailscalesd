package tailscalesd

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestFilterEmpty(t *testing.T) {
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
			got := filterEmpty(tc.in)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("filterEmpty: mismatch (-got +want):\n%v", diff)
			}
		})
	}
}
