BINARY := ./bin/yc-platform-mcp

.PHONY: build run test lint tidy clean

build:
	mkdir -p bin
	go build -o $(BINARY) ./cmd/server

run: build
	OPENAPI_SPEC_PATH=$${OPENAPI_SPEC_PATH:-./specs} $(BINARY)

test:
	go test ./... -count=1 -coverprofile=coverage.out
	go tool cover -func=coverage.out

lint:
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed:"; gofmt -l .; exit 1)
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -rf bin coverage.out
