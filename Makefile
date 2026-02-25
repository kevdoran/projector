BINARY  := pj
MODULE  := ./cmd/projector
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    := $(shell date -u +%Y-%m-%d)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(DATE)

.DEFAULT_GOAL := build
.PHONY: build install test vet tidy clean

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(MODULE)
	@echo "Built ./$(BINARY) — run './$(BINARY) --help' to get started"

install:
	go install -ldflags "$(LDFLAGS)" $(MODULE)

test:
	go test -v -race -count=1 ./...

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -f $(BINARY)
