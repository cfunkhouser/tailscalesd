# TailscaleSD - Prometheus Service Discovery for Tailscale

Serves Prometheus HTTP Service Discovery for devices on a Tailscale Tailnet.

For details on HTTP Service Discovery, read the Prometheus docs:
<https://prometheus.io/docs/prometheus/latest/http_sd/>

## Installation

Release builds for various architectures can be obtained from
[GitHub](https://github.com/cfunkhouser/tailscalesd/releases/latest).

There is also a Docker image provided under `ghcr.io/cfunkhouser/tailscalesd`.
The `latest` tag is automatically updated with each release. The Docker image is
available for `arm/v7`, `arm64` and `amd64`.

## Usage

The `tailscalesd` server is very simple. It serves the SD payload at `/` on its
HTTP server. It respects the following configuration parameters, each of which
may be specified as a flag or an environment variable.

**As of v0.5.0, the local and public APIs are mutually exclusive again.**
The behavior is now that the following flag combinations may not be mixed:

- `-localapi` specifies the local API discovery strategy, using the host node's
  Tailscale local API for discovering peers
- `-token` specifies the public API discovery strategy via API tokens
- `-client_id` + `-client_secret` specifies the public API discovery strategy
  via OAuth credentials

Additionally, the `-tailnet` flag is no longer required, as the Tailscale API
now infers the tailnet from the credentials used for access. It is only required
when the tailnet being discovered is not the default tailnet for the
credentials. It will still be used in target labels, if provided. When not
provided explicitly, the tailnet will be reported as `"-"` in labels.

```console
Usage of tailscalesd:
Most flag value may also be controlled using environment variables. Usage for such flags begins with the variable name.
Explicit flag values take precedent over variable values.

  -a, --address string           (ADDRESS) Address on which to serve Tailscale SD (default "0.0.0.0:9242")
      --client_id string         (TAILSCALE_CLIENT_ID)Tailscale OAuth Client ID
      --client_secret string     (TAILSCALE_CLIENT_SECRET) Tailscale OAuth Client Secret
  -6, --ipv6                     (EXPOSE_IPV6) Include IPv6 target addresses.
  -L, --localapi                 (TAILSCALE_USE_LOCAL_API) Use the Tailscale local API exported by the local node's tailscaled
      --localapi_socket string   (TAILSCALE_LOCAL_API_SOCKET) Unix Domain Socket to use for communication with the local tailscaled API. Safe to omit.
      --log-json                 (LOG_JSON) Output logs in JSON format instead of pretty console format.
  -v, --log-level value          (LOG_LEVEL) Log level to use for output. Defaults to INFO. See log/slog for details.
      --poll duration            (TAILSCALE_API_POLL_LIMIT) Max frequency with which to poll the Tailscale API. Cached results are served between intervals. (default 5m0s)
      --tailnet string           (TAILNET) Tailnet name.
      --token string             (TAILSCALE_API_TOKEN) Tailscale API Token
  -V, --version                  Print the version and exit.
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
- `__meta_tailscale_tailnet`

**As of v0.5.0, handling of Tailscale ACL tags has changed!** For each ACL tag
reported for a device by the Tailscale API, a target label key is created via
the following process:

1. The `tag:` prefix will be removed
2. Value is converted to lower-case / case-folded
3. Any `-` or `:` characters are converted to `_`
4. If at this point the value is empty, the value is set to `EMPTY`
5. Tag value is prepended with `__meta_tailscale_device_tag_`

The value of such a label will always be the static `"1"`. To illustrate, a
device with the ACL tags `tag:prod:1234` and `tag:someService` will result in
the following labels in the discovery payload:

```json
{
  "__meta_tailscale_device_tag_prod_1234": "1",
  "__meta_tailscale_device_tag_someservice": "1"
}
```

**The behavior in which a distinct target descriptor was added for each ACL tag
has been removed in v0.5.0!** All devices now result in a single descriptor.
This handling of ACL tags is new, and it is likely there are edge cases which
are handled poorly, and most likely around unicode tag values. Please file bugs
as issues are discovered.

### Example: Pinging Tailscale Hosts

In the example below, Prometheus will discover Tailscale nodes and attempt to
ping them using a blackbox exporter.

```yaml
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
        regex: "(.*)"
        replacement: $1:9100
        target_label: __address__
```
