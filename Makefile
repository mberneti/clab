MODULE  := github.com/mberneti/clab
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X main.version=$(VERSION)

CMDS := fetch-diff lint-rules post-comments

.PHONY: all build install clean release

all: build

build:
	@mkdir -p bin
	@for cmd in $(CMDS); do \
		echo "  BUILD $$cmd"; \
		go build -ldflags "$(LDFLAGS)" -o bin/clab-$$cmd ./cmd/$$cmd; \
	done

install:
	@for cmd in $(CMDS); do \
		echo "  INSTALL clab-$$cmd → $$(go env GOPATH)/bin/clab-$$cmd"; \
		go install -ldflags "$(LDFLAGS)" ./cmd/$$cmd; \
	done

clean:
	rm -rf bin/

# Cross-platform release archives (used by CI / goreleaser alternative)
release:
	@mkdir -p dist
	@for OS in linux darwin windows; do \
		for ARCH in amd64 arm64; do \
			DIR=dist/clab_$${OS}_$${ARCH}; \
			mkdir -p $$DIR; \
			for cmd in $(CMDS); do \
				EXT=""; [ "$$OS" = "windows" ] && EXT=".exe"; \
				echo "  BUILD $$OS/$$ARCH clab-$$cmd$$EXT"; \
				GOOS=$$OS GOARCH=$$ARCH go build -ldflags "$(LDFLAGS)" \
					-o $$DIR/clab-$$cmd$$EXT ./cmd/$$cmd; \
			done; \
			tar -C dist -czf dist/clab_$${OS}_$${ARCH}.tar.gz clab_$${OS}_$${ARCH}/; \
		done; \
	done

test:
	go test ./...

vet:
	go vet ./...
