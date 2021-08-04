package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/cfunkhouser/tailscalesd"
	"github.com/cfunkhouser/tailscalesd/internal/logwriter"
)

var (
	address   string = "0.0.0.0:9242"
	token     string
	tailnet   string
	printVer  bool
	pollLimit time.Duration = time.Minute * 5

	// Version of tailscalesd. Set at build time to something meaningful.
	Version = "development"
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
	flag.BoolVar(&printVer, "version", false, "Print the version and exit.")
}

func main() {
	log.SetFlags(0)
	log.SetOutput(logwriter.Default())

	defineFlags()
	flag.Parse()

	if printVer {
		fmt.Printf("tailscalesd version %v\n", Version)
		return
	}

	if token == "" || tailnet == "" {
		fmt.Println("Both -token and -tailnet are required.")
		flag.Usage()
		return
	}

	d := tailscalesd.New(tailnet, token, tailscalesd.WithRateLimit(pollLimit))
	http.Handle("/", tailscalesd.Export(d, time.Minute*5))
	log.Printf("Serving Tailscale service discovery on %q", address)
	log.Print(http.ListenAndServe(address, nil))
	log.Print("Done")
}
