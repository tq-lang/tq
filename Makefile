VERSION ?= dev

.PHONY: build test lint check cover clean

build:
	go build -ldflags "-X main.version=$(VERSION)" -o tq ./cmd/tq

test:
	go test -v ./...

lint:
	golangci-lint run

# Run all tests with coverage. CLI integration tests use GOCOVERDIR
# to collect coverage from the cover-built binary subprocess.
cover:
	@rm -rf .coverdata && mkdir -p .coverdata
	go test -count=1 -coverprofile=$(CURDIR)/.coverdata/lib.out \
		./internal/...
	GOCOVERDIR=$(CURDIR)/.coverdata go test -count=1 ./cmd/tq
	@echo ""
	@echo "--- Library coverage (internal packages) ---"
	@go tool cover -func=$(CURDIR)/.coverdata/lib.out | tail -1
	@echo "--- CLI integration coverage (all packages) ---"
	@go tool covdata percent -i=$(CURDIR)/.coverdata

check: lint test build

clean:
	rm -f tq
	rm -rf dist/ .coverdata/
