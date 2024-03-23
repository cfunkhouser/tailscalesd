package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/cfunkhouser/tailscalesd"
)

var (
	address        string = "0.0.0.0:9242"
	includeIPv6    bool
	localAPISocket string        = tailscalesd.PublicAPIHost
	pollLimit      time.Duration = time.Minute * 5
	printVer       bool
	tailnet        string
	token          string
	clientId       string
	clientSecret   string
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
		log.Printf("Duration parsing failed, using default %q: %v", def, err)
	}
	return def
}

func defineFlags() {
	flag.BoolVar(&printVer, "version", false, "Print the version and exit.")
	flag.BoolVar(&includeIPv6, "ipv6", boolEnvVarWithDefault("EXPOSE_IPV6", false), "Include IPv6 target addresses.")
	flag.BoolVar(&useLocalAPI, "localapi", boolEnvVarWithDefault("TAILSCALE_USE_LOCAL_API", false), "Use the Tailscale local API exported by the local node's tailscaled")
	flag.DurationVar(&pollLimit, "poll", durationEnvVarWithDefault("TAILSCALE_API_POLL_LIMIT", pollLimit), "Max frequency with which to poll the Tailscale API. Cached results are served between intervals.")
	flag.StringVar(&address, "address", envVarWithDefault("LISTEN", address), "Address on which to serve Tailscale SD")
	flag.StringVar(&localAPISocket, "localapi_socket", envVarWithDefault("TAILSCALE_LOCAL_API_SOCKET", localAPISocket), "Unix Domain Socket to use for communication with the local tailscaled API.")
	flag.StringVar(&tailnet, "tailnet", os.Getenv("TAILNET"), "Tailnet name.")
	flag.StringVar(&clientId, "client_id", os.Getenv("TAILSCALE_CLIENT_ID"), "Tailscale OAuth Client ID")
	flag.StringVar(&clientSecret, "client_secret", os.Getenv("TAILSCALE_CLIENT_SECRET"), "Tailscale OAuth Client Secret")
	flag.StringVar(&token, "token", os.Getenv("TAILSCALE_API_TOKEN"), "Tailscale API Token")
}

type logWriter struct {
	TZ     *time.Location
	Format string
}

func (w *logWriter) Write(data []byte) (int, error) {
	return fmt.Printf("%v %v", time.Now().In(w.TZ).Format(w.Format), string(data))
}

func main() {
	log.SetFlags(0)
	log.SetOutput(&logWriter{
		TZ:     time.UTC,
		Format: time.RFC3339,
	})

	defineFlags()
	flag.Parse()

	if printVer {
		fmt.Printf("tailscalesd version %v\n", Version)
		return
	}

	hasToken := !(token == "" || tailnet == "")
	hasOAuth := clientId != "" && clientSecret != ""

	if !useLocalAPI && !hasToken && !hasOAuth {
		if _, err := fmt.Fprintln(os.Stderr, "Either -token and -tailnet or -client_id and -client_secret are required when using the public API"); err != nil {
			panic(err)
		}
		flag.Usage()
		return
	}

	if useLocalAPI && localAPISocket == "" {
		if _, err := fmt.Fprintln(os.Stderr, "-localapi_socket must not be empty when using the local API."); err != nil {
			panic(err)
		}
		flag.Usage()
		return
	}

	var ts tailscalesd.MultiDiscoverer
	if useLocalAPI {
		ts = append(ts, &tailscalesd.RateLimitedDiscoverer{
			Wrap:      tailscalesd.LocalAPI(tailscalesd.LocalAPISocket),
			Frequency: pollLimit,
		})
	}

	if token != "" && tailnet != "" {
		ts = append(ts, &tailscalesd.RateLimitedDiscoverer{
			Wrap:      tailscalesd.PublicAPI(tailnet, token),
			Frequency: pollLimit,
		})
	}

	if clientId != "" && clientSecret != "" {
		ts = append(ts, &tailscalesd.RateLimitedDiscoverer{
			Wrap:      tailscalesd.OAuthAPI(clientId, clientSecret),
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

	log.Printf("Serving Tailscale service discovery on %q", address)
	log.Print(http.ListenAndServe(address, nil))
	log.Print("Done")
}
