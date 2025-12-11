package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/common/promslog"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/http"
)

func TestFileServiceDiscovery(t *testing.T) {
	cfg := http.SDConfig{
		HTTPClientConfig: config.DefaultHTTPClientConfig,
		URL:              "http://localhost:9242/",
		RefreshInterval:  model.Duration(60 * time.Second),
	}

	reg := prometheus.NewRegistry()
	refreshMetrics := discovery.NewRefreshMetrics(reg)
	defer refreshMetrics.Unregister()
	metrics := cfg.NewDiscovererMetrics(reg, refreshMetrics)
	if err := metrics.Register(); err != nil {
		t.Fatalf("Failed to register discoverer metrics: %v", err)
	}
	defer metrics.Unregister()

	d, err := http.NewDiscovery(&cfg, promslog.NewNopLogger(), nil, metrics)
	if err != nil {
		t.Fatalf("Failed to create http.Discovery: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tgts, err := d.Refresh(ctx)
	if err != nil {
		t.Fatalf("Failed to refresh targets: %v", err)
	}

	for _, tg := range tgts {
		fmt.Printf("Discovered target: %+v\n", tg.Labels)
	}
}
