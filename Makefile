GO ?= go
BIN := quotary

.PHONY: all build test vet lint docs-check check run clean race

all: check build

build:
	$(GO) build -o $(BIN) ./cmd/quotary

test:
	$(GO) test ./...

race:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

lint:
	$(GO) tool github.com/golangci/golangci-lint/v2/cmd/golangci-lint run

docs-check:
	$(GO) run ./internal/tools/docscheck .

check: vet lint docs-check test

run:
	$(GO) run ./cmd/quotary $(ARGS)

clean:
	rm -f $(BIN)
