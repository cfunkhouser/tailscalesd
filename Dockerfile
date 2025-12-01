# Dockerfile for goreleaser builds
# This is a minimal runtime container that receives the pre-built binary from goreleaser
FROM gcr.io/distroless/static-debian12:nonroot
LABEL maintainer="Simon Elsbrock <simon@iodev.org>"
LABEL org.opencontainers.image.description="Prometheus Service Discovery for Tailscale"

COPY tailscalesd /tailscalesd

ENTRYPOINT ["/tailscalesd"]
CMD []
