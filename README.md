# TailscaleSD - Prometheus Tailscale Service Discovery

Serves Prometheus HTTP Service Discovery for devices on a Tailscale Tailnet.

For details on HTTP Service Discovery, read the Prometheus docs:
https://prometheus.io/docs/prometheus/latest/http_sd/

## Usage

The `tailscalesd` server is very simple. It serves the SD payload at `/` on its
HTTP server. It respects only three configuration parameters, each of which may
be specified as a flag or an environment variable.

- `--token` / `TAILSCALE_API_TOKEN` is a Tailscale API token with appropriate
  permissions to access the Tailscale API and iterate devices.
- `--tailnet` / `TAILNET` is the name of the tailnet to enumerate.
- `--address` / `ADDRESS` is the host:port on which to serve Tailscale SD.
  Defaults to `0.0.0.0:9242`.

```console
$ TOKEN=SUPERSECRET tailscalesd --tailnet alice@gmail.com
2021/08/03 16:00:38 Serving Tailscale service discovery on "0.0.0.0:9242"
```

## Prometheus Configuration

Configure Prometheus by placing the `tailscalesd` URL in a `http_sd_configs`
block in a `scrape_config`. The following labels are potentially made available
for all Tailscale nodes discovered, however any label for which the Tailscale
API did not return a value will be omitted. For more details on each field and
the API in general, see:
https://github.com/tailscale/tailscale/blob/main/api.md#tailnet-devices-get

Possible target labels:

- `__meta_tailscale_device_authorized`
- `__meta_tailscale_device_client_version`
- `__meta_tailscale_device_hostname`
- `__meta_tailscale_device_id`
- `__meta_tailscale_device_is_external`
- `__meta_tailscale_device_machine_key`
- `__meta_tailscale_device_name`
- `__meta_tailscale_device_node_key`
- `__meta_tailscale_device_os`
- `__meta_tailscale_device_user`

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