.PHONY: build build-all clean run test

APP_NAME := care-scanner-bridge
VERSION := 1.0.0

# Build for current platform
build:
	go build -ldflags="-s -w -X main.Version=$(VERSION)" -o $(APP_NAME) .

# Build for all platforms
build-all: build-darwin-amd64 build-darwin-arm64 build-linux-amd64 build-windows-amd64

build-darwin-amd64:
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 go build -ldflags="-s -w" -o dist/$(APP_NAME)-darwin-amd64 .

build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 go build -ldflags="-s -w" -o dist/$(APP_NAME)-darwin-arm64 .

build-linux-amd64:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -ldflags="-s -w" -o dist/$(APP_NAME)-linux-amd64 .

build-windows-amd64:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=1 go build -ldflags="-s -w" -o dist/$(APP_NAME)-windows-amd64.exe .

# Run the application
run:
	go run .

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -f $(APP_NAME)
	rm -rf dist/

# Download dependencies
deps:
	go mod download
	go mod tidy

# Create macOS universal binary
universal-darwin: build-darwin-amd64 build-darwin-arm64
	lipo -create -output dist/$(APP_NAME)-darwin-universal dist/$(APP_NAME)-darwin-amd64 dist/$(APP_NAME)-darwin-arm64

# Install locally (macOS/Linux)
install: build
	mkdir -p ~/bin
	cp $(APP_NAME) ~/bin/
	@echo "Installed to ~/bin/$(APP_NAME)"
