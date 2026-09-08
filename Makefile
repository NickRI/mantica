.PHONY: build release-artifacts clean

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo 0.1.0)
DIST ?= dist

build:
	mkdir -p $(DIST)
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o $(DIST)/mantica .

release-artifacts: build
	cd $(DIST) && tar -czf mantica-linux-amd64.tar.gz mantica
	printf '%s\n' "$(VERSION)" > $(DIST)/VERSION

clean:
	rm -rf $(DIST)
