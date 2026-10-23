BINARY_NAME=cmc
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-s -w -X github.com/openCMC/CoinMarketCap-CLI/cmd.version=$(VERSION) -X github.com/openCMC/CoinMarketCap-CLI/cmd.commit=$(COMMIT) -X github.com/openCMC/CoinMarketCap-CLI/cmd.date=$(DATE)"

.PHONY: build test lint clean spec

build:
	go build $(LDFLAGS) -o $(BINARY_NAME) .

test:
	go test -race ./...

lint:
	golangci-lint run ./...

clean:
	rm -f $(BINARY_NAME)

# Regenerate the embedded operation index from the published OpenAPI spec.
spec:
	go run ./internal/spec/gen -out internal/spec/operations.json
