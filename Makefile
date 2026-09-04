BINARY := ploi-tui
MAIN := ./cmd/ploi-tui
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64

.PHONY: all build run test vet lint cover cross snapshot clean

all: lint test build

build:
	CGO_ENABLED=0 go build -trimpath \
		-ldflags "-s -w -X main.version=$(VERSION)" \
		-o bin/$(BINARY) $(MAIN)

run: build
	./bin/$(BINARY)

test:
	go test -race ./...

vet:
	go vet ./...

lint:
	golangci-lint run

cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

cross:
	mkdir -p dist/manual
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; arch=$${platform#*/}; \
		echo "== $${os}/$${arch} =="; \
		CGO_ENABLED=0 GOOS=$${os} GOARCH=$${arch} go build -trimpath \
			-ldflags "-s -w -X main.version=$(VERSION)" \
			-o dist/manual/$(BINARY)-$${os}-$${arch} $(MAIN) || exit 1; \
	done

snapshot:
	goreleaser release --snapshot --clean

clean:
	rm -rf bin dist coverage.out coverage.html
