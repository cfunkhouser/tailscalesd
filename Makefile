.PHONY = all clean

PLATFORMS := linux-arm6 linux-arm7 linux-amd64 linux-386 darwin-amd64
DEBARCHS := amd64 armhf arm64 i386
VERSION := $(shell git describe --always --tags --dirty="-dev-$$(git rev-parse --short HEAD)" | sed -e '1s/^.//')
MAIN := ./cmd/tailscalesd

BUILDCMD := go build -o
ifneq ($(strip $(VERSION)),)
	BUILDCMD := go build -ldflags="-X 'main.Version=$(VERSION)'" -o
endif

DISTBINS := $(foreach ku,$(PLATFORMS),tailscalesd-$(ku))
SUMS := SHA1SUM.txt SHA256SUM.txt

ifeq ($(strip $(GOARCH)),)
	GOARCH := $(shell go env GOARCH)
endif
ifeq ($(strip $(GOARM)),)
	GOARM := $(shell go env GOARM)
endif
ifeq ($(strip $(GOOS)),)
	GOOS := $(shell go env GOOS)
endif

ARCH := $(GOARCH)
DEBARCH := $(GOARCH)
ifeq ($(strip $(GOOS)), linux)
	ifeq ($(strip $(GOARCH)), arm)
		ARCH = arm$(GOARM)
		DEBARCH = armhf
	endif
endif
ifeq ($(strip $(DEBARCH)),386)
	DEBARCH := i386
endif

DEBPACKAGE := tailscalesd_$(VERSION)_$(DEBARCH)

all: tailscalesd

deb: $(DEBPACKAGE).deb

clean:
	@rm -fv $(DISTBINS) $(SUMS) tailscalesd
	@rm -rfv tailscalesd_$(VERSION)*

test:
	go test -v ./... -bench=.

tailscalesd:
	$(BUILDCMD) $@ $(MAIN)

dist: $(DISTBINS) $(SUMS)
	@chmod +x $(DISTBINS)

tailscalesd-linux-arm%:
	env GOOS=linux GOARCH=arm GOARM=$* $(BUILDCMD) $@ $(MAIN)

tailscalesd-linux-%:
	env GOOS=linux GOARCH=$* $(BUILDCMD) $@ $(MAIN)

tailscalesd-darwin-%:
	env GOOS=darwin GOARCH=$* $(BUILDCMD) $@ $(MAIN)

SHA%SUM.txt: $(DISTBINS)
	shasum -a $* $(DISTBINS) > $@

tailscalesd_$(VERSION)_$(DEBARCH): tailscalesd-linux-$(GOARCH)
# $(DEBPACKAGE): tailscalesd
	mkdir -vp $@/usr/bin
	cp -rfv ./package/* $@/
	cp -rfv ./tailscalesd-linux-$(GOARCH) $@/usr/bin/tailscalesd

tailscalesd_$(VERSION)_$(DEBARCH)/DEBIAN/control: tailscalesd_$(VERSION)_$(DEBARCH)
	cp ./package/DEBIAN/control $@
	sed -i'' "s/%VERSION%/$(VERSION)/g" $@
	sed -i'' "s/%ARCH%/$(DEBARCH)/g" $@
	sed -i'' "s/%SIZE%/$(shell du -s $(DEBPACKAGE) | cut -f1 | xargs echo ".001 *" | bc | xargs printf "%.0f\n")/g" $@

tailscalesd_$(VERSION)_$(DEBARCH).deb: tailscalesd_$(VERSION)_$(DEBARCH)/DEBIAN/control
	fakeroot dpkg-deb --build tailscalesd_$(VERSION)_$(DEBARCH)