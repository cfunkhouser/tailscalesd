package tailscalesd

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	apiRequestCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tailscalesd_tailscale_api_requests",
			Help: "Counter of requests made to Tailscale APIs. Labeled with the API host to which requests are made.",
		},
		[]string{"api", "host"})

	apiRequestLatencyHistogram = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "tailscalesd_tailscale_api_request_latency_ms",
			Help: "Histogram of API request latency measured in milliseconds. " +
				"Bucketted geometrically.",
			Buckets: []float64{1, 2.75, 7.5625, 20.7969, 57.1914, 157.2764, 432.5100, 1189.4025, 3270.8569, 8994.8566},
		},
		[]string{"api", "host"})

	apiRequestErrorCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tailscalesd_tailscale_api_errors",
			Help: "Counter of errors during requests to Tailscale APIs. " +
				"Denominated by tailscalesd_tailscale_api_requests.",
		},
		[]string{"api", "host"})

	apiPayloadErrorCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tailscalesd_tailscale_api_payload_errors",
			Help: "Counter of bad payload responses from Tailscale APIs. Denominated by tailscalesd_tailscale_api_requests.",
		},
		[]string{"api", "host"})

	multiDiscovererRequestCounter = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "tailscalesd_tailscale_multi_requests",
			Help: "Counter of all requests to a multi-discoverer.",
		})

	multiDiscovererErrorCounter = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "tailscalesd_tailscale_multi_errors",
			Help: "Counter of errors during requests to all multi-discoverer. " +
				"Denominated by tailscalesd_tailscale_multi_requests.",
		})

	rateLimitedRequests = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "tailscalesd_tailscale_rate_limited_requests",
			Help: "Counter of all requests to a rate limited discoverer.",
		})

	rateLimitedRequestRefreshses = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "tailscalesd_tailscale_rate_limited_refreshes",
			Help: "Counter of requests to a rate limited discoverer which result in a data refresh.",
		})

	rateLimitedStaleResults = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "tailscalesd_tailscale_rate_limited_stale",
			Help: "Counter of requests to a rate limited discoverer which result a return of stale results.",
		})

	tailnetDevicesFoundCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tailscalesd_public_api_devices_found",
			Help: "Counter of devices found using the public API, labeled with tailnet name.",
		},
		[]string{"tailnet"})
)
