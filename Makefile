.PHONY: build release-artifacts clean

VERSION ?= 0.1.7
COMMIT  ?= $(shell git rev-parse --short=12 HEAD 2>/dev/null || echo unknown)
DIST    ?= dist
LDFLAGS  = -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)

build:
	mkdir -p $(DIST)
	CGO_ENABLED=1 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST)/mantica .

release-artifacts: build
	cd $(DIST) && tar -czf mantica-linux-amd64.tar.gz mantica
	printf '%s\n' "$(VERSION)" > $(DIST)/VERSION
	printf '%s\n' "$(COMMIT)" > $(DIST)/COMMIT

clean:
	rm -rf $(DIST)
