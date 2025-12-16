package main

import (
	"context"
	"log/slog"

	"tailscale.com/client/local"
)

func main() {
	var client local.Client
	ctx := context.Background()

	status, err := client.Status(ctx)
	if err != nil {
		slog.Error("Failed getting status from Tailscale local API", "error", err)
	}

	for _, peer := range status.Peer {
		slog.Info("Found peer", "hostname", peer.HostName, "peer", peer)
	}
}
