FROM golang:1.13 AS builder
LABEL maintainer="Simon Elsbrock <simon@iodev.org>"
LABEL org.opencontainers.image.description="Prometheus Service Discovery for Tailscale"

COPY . ./build/tailscalesd/
RUN cd ./build/tailscalesd && make

FROM golang:1.13
COPY --from=builder /go/build/tailscalesd/tailscalesd /tailscalesd

ENTRYPOINT ["/tailscalesd"]
CMD []