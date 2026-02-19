.PHONY: build test lint check clean

build:
	go build -o tq ./cmd/tq

test:
	go test ./...

lint:
	golangci-lint run

check: lint test build

clean:
	rm -f tq
	rm -rf dist/
