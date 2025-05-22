# Study Accountability Bot Makefile

# Binary name
BINARY_NAME=accountabot

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOMOD=$(GOCMD) mod
GOGET=$(GOCMD) get

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	$(GOBUILD) -o $(BINARY_NAME) -v

# Run the bot (builds first if needed)
run: build
	@echo "Starting $(BINARY_NAME)..."
	./$(BINARY_NAME)

# Initialize Go modules and download dependencies
init:
	@echo "Initializing Go module..."
	$(GOMOD) init accountabot
	@echo "Downloading dependencies..."
	$(GOGET) github.com/bwmarrin/discordgo

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	$(GOCLEAN)
	rm -f $(BINARY_NAME)

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Build for different platforms
build-linux:
	@echo "Building for Linux..."
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_NAME)-linux -v

build-windows:
	@echo "Building for Windows..."
	GOOS=windows GOARCH=amd64 $(GOBUILD) -o $(BINARY_NAME)-windows.exe -v

build-mac:
	@echo "Building for macOS..."
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -o $(BINARY_NAME)-mac -v

# Build for all platforms
build-all: build-linux build-windows build-mac
	@echo "Built for all platforms"

# Development run with auto-restart (requires 'air' tool)
dev:
	@echo "Starting development mode..."
	@if command -v air > /dev/null; then \
		air; \
	else \
		echo "Air not installed. Install with: go install github.com/cosmtrek/air@latest"; \
		echo "Falling back to regular run..."; \
		make run; \
	fi

# Install development tools
install-dev-tools:
	@echo "Installing development tools..."
	go install github.com/cosmtrek/air@latest

# Check if config.json exists
check-config:
	@if [ ! -f config.json ]; then \
		echo "❌ config.json not found!"; \
		echo "Please create config.json with your Discord bot token and channel ID"; \
		echo "See README.md for configuration details"; \
		exit 1; \
	else \
		echo "✅ config.json found"; \
	fi

# Safe run that checks config first
safe-run: check-config run

# Display help
help:
	@echo "Available commands:"
	@echo "  make build          - Build the bot binary"
	@echo "  make run            - Build and run the bot"
	@echo "  make safe-run       - Check config and run the bot"
	@echo "  make init           - Initialize Go module and download dependencies"
	@echo "  make clean          - Clean build artifacts"
	@echo "  make deps           - Download and tidy dependencies"
	@echo "  make build-linux    - Build for Linux"
	@echo "  make build-windows  - Build for Windows"
	@echo "  make build-mac      - Build for macOS"
	@echo "  make build-all      - Build for all platforms"
	@echo "  make dev            - Run in development mode (auto-restart)"
	@echo "  make install-dev-tools - Install development tools"
	@echo "  make check-config   - Check if config.json exists"
	@echo "  make help           - Show this help message"

# Default target
.DEFAULT_GOAL := help

# Declare phony targets (targets that don't create files)
.PHONY: build run init clean deps build-linux build-windows build-mac build-all dev install-dev-tools check-config safe-run help