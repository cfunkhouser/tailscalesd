// tailscalesd is a Prometheus service discovery exporter for tailnets.
package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/lmittmann/tint"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/pflag"
	"tailscale.com/client/tailscale/v2"

	"github.com/cfunkhouser/tailscalesd"
)

var (
	address   = "0.0.0.0:9242"
	pollLimit = time.Minute * 5

	clientID       string
	clientSecret   string
	includeIPv6    bool
	localAPISocket string
	logLevel       slog.LevelVar
	logJSON        bool
	printVer       bool
	tailnet        string
	token          string
	useLocalAPI    bool

	// Version of tailscalesd. Set at build time to something meaningful.
	Version = "development"
)

func envVarWithDefault(key, def string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}

	return def
}

func getAndUnsetEnv(key string) string {
	val, ok := os.LookupEnv(key)
	if ok {
		if err := os.Unsetenv(key); err != nil {
			slog.Debug("Failed unsetting environment variable", "key", key, "error", err)
		}
	}

	return val
}

func boolEnvVarWithDefault(key string, def bool) bool {
	if val, ok := os.LookupEnv(key); ok {
		val = strings.ToLower(strings.TrimSpace(val))
		return val == "true" || val == "yes"
	}

	return def
}

func durationEnvVarWithDefault(key string, def time.Duration) time.Duration {
	if val, ok := os.LookupEnv(key); ok {
		d, err := time.ParseDuration(val)
		if err == nil {
			return d
		}

		slog.Warn("Failed parsing duration, using default", "default", def, "error", err)
	}

	return def
}

func setLevelFlagValue(l string) error {
	return logLevel.UnmarshalText([]byte(l))
}

func defineFlags() {
	pflag.BoolVarP(&printVer, "version", "V", false, "Print the version and exit.")
	pflag.BoolVarP(&includeIPv6, "ipv6", "6", boolEnvVarWithDefault("EXPOSE_IPV6", false), "Include IPv6 target addresses.")
	pflag.BoolVarP(&useLocalAPI, "localapi", "L", boolEnvVarWithDefault("TAILSCALE_USE_LOCAL_API", false), "Use the Tailscale local API exported by the local node's tailscaled")
	pflag.DurationVar(&pollLimit, "poll", durationEnvVarWithDefault("TAILSCALE_API_POLL_LIMIT", pollLimit), "Max frequency with which to poll the Tailscale API. Cached results are served between intervals.")
	pflag.StringVarP(&address, "address", "a", envVarWithDefault("LISTEN", address), "Address on which to serve Tailscale SD")
	pflag.StringVar(&localAPISocket, "localapi_socket", envVarWithDefault("TAILSCALE_LOCAL_API_SOCKET", localAPISocket), "Unix Domain Socket to use for communication with the local tailscaled API. Safe to omit.")
	pflag.StringVar(&tailnet, "tailnet", os.Getenv("TAILNET"), "Tailnet name.")
	pflag.StringVar(&clientID, "client_id", os.Getenv("TAILSCALE_CLIENT_ID"), "Tailscale OAuth Client ID")
	pflag.StringVar(&clientSecret, "client_secret", getAndUnsetEnv("TAILSCALE_CLIENT_SECRET"), "Tailscale OAuth Client Secret")
	pflag.StringVar(&token, "token", getAndUnsetEnv("TAILSCALE_API_TOKEN"), "Tailscale API Token")
	pflag.BoolVar(&logJSON, "log-json", boolEnvVarWithDefault("LOG_JSON", false), "Output logs in JSON format instead of pretty console format.")
	pflag.FuncP("log-level", "v", "Log level to use for output. Defaults to INFO. See log/slog for details.", setLevelFlagValue)
}

func main() {
	defineFlags()
	pflag.Parse()

	var h slog.Handler
	if logJSON {
		h = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: &logLevel,
		})
	} else {
		h = tint.NewHandler(os.Stderr, &tint.Options{
			Level:      &logLevel,
			TimeFormat: time.RFC3339,
		})
	}

	slog.SetDefault(slog.New(h))

	if printVer {
		fmt.Printf("tailscalesd version %v\n", Version)
		return
	}

	hasOAuth := clientID != "" && clientSecret != ""
	if !useLocalAPI && token == "" && !hasOAuth {
		if _, err := fmt.Fprintln(os.Stderr, "Either -token and -tailnet or -client_id and -client_secret are required when using the public API"); err != nil {
			panic(err)
		}
		pflag.Usage()
		return
	}

	var ts tailscalesd.MultiDiscoverer
	if useLocalAPI {
		ts = append(ts, &tailscalesd.RateLimitedDiscoverer{
			Wrap:      tailscalesd.LocalAPI(localAPISocket),
			Frequency: pollLimit,
		})
	}

	if token != "" {
		ts = append(ts, &tailscalesd.RateLimitedDiscoverer{
			Wrap: &tailscalesd.TailscaleAPIDiscoverer{
				Client: &tailscale.Client{
					APIKey:  token,
					Tailnet: tailnet,
				},
			},
			Frequency: pollLimit,
		})
	}

	if clientID != "" && clientSecret != "" {
		ts = append(ts, &tailscalesd.RateLimitedDiscoverer{
			Wrap: &tailscalesd.TailscaleAPIDiscoverer{
				Client: &tailscale.Client{
					Auth: &tailscale.OAuth{
						ClientID:     clientID,
						ClientSecret: clientSecret,
						Scopes:       []string{"devices:core:read"},
					},
					Tailnet: tailnet,
				},
			},
			Frequency: pollLimit,
		})
	}

	var filters []tailscalesd.TargetFilter
	if !includeIPv6 {
		filters = append(filters, tailscalesd.FilterIPv6Addresses)
	}

	// Metrics concerning tailscalesd itself are served from /metrics
	http.Handle("/metrics", promhttp.Handler())
	// Service discovery is served at /
	http.Handle("/", tailscalesd.Export(ts, filters...))

	slog.Info("Serving Tailscale service discovery", "address", address)
	server := &http.Server{
		Addr:         address,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		slog.Warn("Server stopped with unexpected error", "error", err)
	}
	slog.Debug("Tailscale service discovery done")
}
