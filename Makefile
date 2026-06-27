# Kolibri Mesh Makefile

# Build variables
BINARY_DIR=bin
COORDINATOR_BIN=coordinator
AGENT_BIN=agent
CLI_BIN=mesh

# Go variables
GO=go
GOFLAGS=-v
LDFLAGS=-s -w

.PHONY: all build clean test lint install

# Build all binaries
all: build

# Build specific binaries
build: coordinator agent cli

coordinator:
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY_DIR)/$(COORDINATOR_BIN) ./cmd/coordinator

agent:
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY_DIR)/$(AGENT_BIN) ./cmd/agent

cli:
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY_DIR)/$(CLI_BIN) ./cmd/cli

# Clean build artifacts
clean:
	rm -rf $(BINARY_DIR)

# Run tests
test:
	$(GO) test ./...

# Run linter
lint:
	golangci-lint run

# Install binaries
install: build
	cp $(BINARY_DIR)/$(CLI_BIN) /usr/local/bin/
	cp $(BINARY_DIR)/$(COORDINATOR_BIN) /usr/local/bin/
	cp $(BINARY_DIR)/$(AGENT_BIN) /usr/local/bin/

# Build for all platforms
build-all:
	GOOS=linux GOARCH=amd64 $(GO) build -o $(BINARY_DIR)/$(COORDINATOR_BIN)-linux-amd64 ./cmd/coordinator
	GOOS=linux GOARCH=amd64 $(GO) build -o $(BINARY_DIR)/$(AGENT_BIN)-linux-amd64 ./cmd/agent
	GOOS=linux GOARCH=amd64 $(GO) build -o $(BINARY_DIR)/$(CLI_BIN)-linux-amd64 ./cmd/cli
	GOOS=darwin GOARCH=arm64 $(GO) build -o $(BINARY_DIR)/$(COORDINATOR_BIN)-darwin-arm64 ./cmd/coordinator
	GOOS=darwin GOARCH=arm64 $(GO) build -o $(BINARY_DIR)/$(AGENT_BIN)-darwin-arm64 ./cmd/agent
	GOOS=darwin GOARCH=arm64 $(GO) build -o $(BINARY_DIR)/$(CLI_BIN)-darwin-arm64 ./cmd/cli

# Docker build
docker:
	docker build -t kolibri-mesh .

# Help
help:
	@echo "Kolibri Mesh - Build targets:"
	@echo "  all          - Build all binaries"
	@echo "  build        - Build coordinator, agent, and cli"
	@echo "  coordinator  - Build coordinator binary"
	@echo "  agent        - Build agent binary"
	@echo "  cli          - Build CLI binary"
	@echo "  clean        - Remove build artifacts"
	@echo "  test         - Run tests"
	@echo "  lint         - Run linter"
	@echo "  install      - Install binaries to /usr/local/bin"
	@echo "  build-all    - Build for all platforms"
	@echo "  docker       - Build Docker image"
	@echo "  help         - Show this help"
