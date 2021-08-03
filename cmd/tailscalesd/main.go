package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/cfunkhouser/tailscalesd"
)

var (
	address string = "0.0.0.0:9242"
	token   string
	tailnet string
)

func envVarWithDefault(key, def string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return def
}

func defineFlags() {
	flag.StringVar(&address, "address", envVarWithDefault("LISTEN", address), "Address on which to serve Tailscale SD")
	flag.StringVar(&token, "token", os.Getenv("TAILSCALE_API_TOKEN"), "Tailscale API Token")
	flag.StringVar(&tailnet, "tailnet", os.Getenv("TAILNET"), "Tailnet name.")
}

func main() {
	defineFlags()
	flag.Parse()
	if token == "" || tailnet == "" {
		log.Fatal("Both --token and --tailnet are required.")
	}
	d := tailscalesd.New(tailnet, token)
	http.Handle("/", tailscalesd.Export(d, time.Minute*5))
	log.Printf("Serving Tailscale service discovery on %q", address)
	log.Print(http.ListenAndServe(address, nil))
	log.Print("Done")
}
