VERSION ?= dev

.PHONY: build test test-docs lint check cover clean changelog check-changelog

build:
	go build -ldflags "-X main.version=$(VERSION) -X main.commit=$(shell git rev-parse --short HEAD) -X main.date=$(shell date -u +%Y-%m-%d)" -o tq ./cmd/tq

test:
	go test -v ./...

changelog:
	git-cliff -o CHANGELOG.md

check-changelog:
	git-cliff -o CHANGELOG.md
	git diff --exit-code -- CHANGELOG.md

test-docs:
	go test -v -run TestDocs ./cmd/tq

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
