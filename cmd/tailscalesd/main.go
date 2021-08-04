package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/cfunkhouser/tailscalesd"
	"github.com/cfunkhouser/tailscalesd/internal/logwriter"
)

var (
	address string = "0.0.0.0:9242"
	token   string
	tailnet string

	pollLimit time.Duration = time.Minute * 5
)

func envVarWithDefault(key, def string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
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
	flag.StringVar(&address, "address", envVarWithDefault("LISTEN", address), "Address on which to serve Tailscale SD")
	flag.StringVar(&token, "token", os.Getenv("TAILSCALE_API_TOKEN"), "Tailscale API Token")
	flag.StringVar(&tailnet, "tailnet", os.Getenv("TAILNET"), "Tailnet name.")
	flag.DurationVar(&pollLimit, "poll", durationEnvVarWithDefault("TAILSCALE_API_POLL_LIMIT", pollLimit), "Max frequency with which to poll the Tailscale API. Cached results are served between intervals.")
}

func main() {
	log.SetFlags(0)
	log.SetOutput(logwriter.Default())

	defineFlags()
	flag.Parse()
	if token == "" || tailnet == "" {
		log.Fatal("Both --token and --tailnet are required.")
	}

	d := tailscalesd.New(tailnet, token, tailscalesd.WithRateLimit(pollLimit))
	http.Handle("/", tailscalesd.Export(d, time.Minute*5))
	log.Printf("Serving Tailscale service discovery on %q", address)
	log.Print(http.ListenAndServe(address, nil))
	log.Print("Done")
}
