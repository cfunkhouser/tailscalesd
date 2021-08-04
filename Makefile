.PHONY = all clean

PLATFORMS := linux-arm6 linux-arm7 linux-amd64 linux-386 darwin-amd64
VERSION := $(shell git describe --always --tags --dirty="-dev-$$(git rev-parse --short HEAD)")
MAIN := ./cmd/tailscalesd

BUILDCMD := go build -o
ifneq ($(strip $(VERSION)),)
	BUILDCMD := go build -ldflags="-X 'main.Version=$(VERSION)'" -o
endif


DISTTARGETS := $(foreach ku,$(PLATFORMS),tailscalesd-$(ku))
SUMS := SHA1SUM.txt SHA256SUM.txt

all: tailscalesd

test:
	go test -v ./... -bench=.

tailscalesd:
	$(BUILDCMD) $@ $(MAIN)

dist: $(DISTTARGETS) $(SUMS)
	@chmod +x $(DISTTARGETS)

tailscalesd-linux-arm%:
	env GOOS=linux GOARCH=arm GOARM=$* $(BUILDCMD) $@ $(MAIN)

tailscalesd-linux-%:
	env GOOS=linux GOARCH=$* $(BUILDCMD) $@ $(MAIN)

tailscalesd-darwin-%:
	env GOOS=darwin GOARCH=$* $(BUILDCMD) $@ $(MAIN)

SHA%SUM.txt: $(DISTTARGETS)
	shasum -a $* $(DISTTARGETS) > $@

clean:
	@rm -fv $(DISTTARGETS) $(SUMS) tailscalesd
