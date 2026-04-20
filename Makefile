.PHONY: build test cover lint vuln e2e all clean

BIN := bin/envlens

build:
	go build -o $(BIN) ./cmd/envlens

test:
	go test -race ./internal/... ./cmd/...

cover:
	go test -race -coverprofile=coverage.out -covermode=atomic ./internal/... ./cmd/...
	go tool cover -html=coverage.out

lint:
	golangci-lint run

vuln:
	go install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck ./...

e2e:
	go test -race ./e2e/...

all: lint vuln test e2e

clean:
	rm -rf bin dist coverage.out e2e/testdata/envlens e2e/testdata/envlens.exe
