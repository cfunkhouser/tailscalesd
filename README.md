# TailscaleSD - Prometheus Service Discovery for Tailscale

Serves Prometheus HTTP Service Discovery for devices on a Tailscale Tailnet.

For details on HTTP Service Discovery, read the Prometheus docs:
<https://prometheus.io/docs/prometheus/latest/http_sd/>

## Installation

Release builds for various architectures can be obtained from
[GitHub](https://github.com/cfunkhouser/tailscalesd/releases/latest).

There is also a
Docker image provided under `ghcr.io/cfunkhouser/tailscalesd`. The `latest` tag
is automatically updated with each release. The Docker image is available for
`arm/v7`, `arm64` and `amd64`.

## Usage

The `tailscalesd` server is very simple. It serves the SD payload at `/` on its
HTTP server. It respects the following configuration parameters, each of which
may be specified as a flag or an environment variable.

**As of v0.2.1 the the local and public APIs are no longer mutually exclusive.
Setting the `-localapi` flag and providing `-tailnet` + `-token` will result in
a union of targets from both APIs.**

- `-address` / `ADDRESS` is the host:port on which to serve TailscaleSD.
  Defaults to `0.0.0.0:9242`.
- `-ipv6` / `EXPOSE_IPV6` instructs TailscaleSD to include IPv6 addresses in the
  target list. **Be careful with this, the colons in IPv6 addresses wreak havoc
  with Prometheus configurations!**
- `-localapi` / `TAILSCALE_USE_LOCAL_API` instructs TailscaleSD to use the
  `tailscaled`-exported local API for discovery.
- `-localapi_socket` / `TAILSCALE_LOCAL_API_SOCKET` is the path to the Unix
  domain socket over which `tailscaled` serves the local API.
- `-poll` / `TAILSCALE_API_POLL_LIMIT` is the limit of how frequently the
  Tailscale API may be polled. Cached results are served between intervals.
  Defaults to 5 minutes. Also applies to local API.
- `-tailnet` / `TAILNET` is the name of the tailnet to enumerate. **No longer
  required when using the public API.** If omitted, the placeholder default
  `"-"` will be used, which is consistent with the new behavior of the Tailscale
  API.
- `-token` / `TAILSCALE_API_TOKEN` is a Tailscale API token with appropriate
  permissions to access the Tailscale API and enumerate devices. Mutually
  exclusive with `-client_id` / `-client_secret`.
- `-client_id` / `TAILSCALE_CLIENT_ID` is an OAuth Client ID that can be used to
  get scoped Tailscale API access, and needn't be as short-lived as Tailscale
  API tokens. It must be used with `-client_secret`.
- `-client_secret` / `TAILSCALE_CLIENT_SECRET` is an OAuth Client Secret that
  can be used to get scoped Tailscale API access, and needn't be as short-lived
  as Tailscale API tokens. It must be used with `-client_id`

```console
$ TAILSCALE_API_TOKEN=SUPERSECRET tailscalesd
2025-12-02T15:38:14Z Serving Tailscale service discovery on "0.0.0.0:9242"
```

### Public vs Local API

TailscaleSD is capable of discovering devices both from Tailscale's public API,
and from the local API served by `tailscaled` on the node on which TailscaleSD
is run. By using the public API, TailscaleSD will dicover _all_ devices in the
tailnet, regardless of whether the local node is able to reach them or not.
Devices found using the local API will be reachable from the local node,
according to your Tailscale ACLs.

See the label comments in [`tailscalesd.go`](./tailscalesd.go) for details about
which labels are supported for each API type. **Do not assume they will be the
same labels, or that values will match across the APIs!**

## Metrics

As of v0.2.1, TailscaleSD exports Prometheus metrics on the standard `/metrics`
endpoint. In addition to the standard Go metrics, you will find
TailscaleSD-specific metrics defined in [`metrics.go`](./metrics.go). The
metrics are targetted at understanding the behavior of TailscaleSD itself.
Contributions of additional interesting metrics are welcome, but please remember
that details about your devices should be handled by your monitoring. This is a
target discovery tool, _not_ a Prometheus exporter for Tailscale!

## Prometheus Configuration

Configure Prometheus by placing the `tailscalesd` URL in a `http_sd_configs`
block in a `scrape_config`. The following labels are potentially made available
for all Tailscale nodes discovered, however any label for which the Tailscale
API did not return a value will be omitted. For more details on each field and
the API in general, see:
<https://github.com/tailscale/tailscale/blob/main/api.md#tailnet-devices-get>

Possible target labels follow. See the label comments in
[`tailscalesd.go`](./tailscalesd.go) for details. There will be one target entry
for each unique combination of all labels.

- `__meta_tailscale_api`
- `__meta_tailscale_device_authorized`
- `__meta_tailscale_device_client_version`
- `__meta_tailscale_device_hostname`
- `__meta_tailscale_device_id`
- `__meta_tailscale_device_name`
- `__meta_tailscale_device_os`
- `__meta_tailscale_device_tag`
- `__meta_tailscale_tailnet`

### Example: Pinging Tailscale Hosts

In the example below, Prometheus will discover Tailscale nodes and attempt to
ping them using a blackbox exporter.

```yaml
---
global:
  scrape_interval: 1m
scrape_configs:
- job_name: tailscale-prober
    metrics_path: /probe
    params:
      module: [icmp]
    http_sd_configs:
      - url: http://localhost:9242/
    relabel_configs:
      - source_labels: [__address__]
        target_label: __param_target
      - source_labels: [__param_target]
        target_label: instance
      - source_labels: [__meta_tailscale_device_hostname]
        target_label: tailscale_hostname
      - source_labels: [__meta_tailscale_device_name]
        target_label: tailscale_name
      - target_label: __address__
        replacement: your.blackbox.exporter:9115
```

### Example: Scraping Node Exporter from Tailscale Hosts

This example appends the node exporter port `9100` to the addresses returned
from the Tailscale API, and instructs Prometheus to collect those metrics. This
is likely to result in many "down" targets if your tailnet contains hosts
without the node exporter. It also doesn't play well with IPv6 addresses.

```yaml
---
global:
  scrape_interval: 1m
scrape_configs:
- job_name: tailscale-node-exporter
    http_sd_configs:
      - url: http://localhost:9242/
    relabel_configs:
      - source_labels: [__meta_tailscale_device_hostname]
        target_label: tailscale_hostname
      - source_labels: [__meta_tailscale_device_name]
        target_label: tailscale_name
      - source_labels: [__address__]
        regex: '(.*)'
        replacement: $1:9100
        target_label: __address__
```
