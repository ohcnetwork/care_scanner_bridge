.PHONY: build build-all clean run test app-bundle dmg

APP_NAME := care-scanner-bridge
APP_DISPLAY_NAME := Care Scanner Bridge
VERSION := 1.0.0
BUNDLE_ID := com.ohcnetwork.care-scanner-bridge

# Directories
DIST_DIR := dist
BUILD_DIR := build/darwin
APP_BUNDLE := $(DIST_DIR)/$(APP_DISPLAY_NAME).app

# Build for current platform
build:
	go build -ldflags="-s -w -X main.Version=$(VERSION)" -o $(APP_NAME) .

# Build for all platforms
build-all: build-darwin-amd64 build-darwin-arm64 build-linux-amd64 build-windows-amd64

build-darwin-amd64:
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 go build -ldflags="-s -w" -o $(DIST_DIR)/$(APP_NAME)-darwin-amd64 .

build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 go build -ldflags="-s -w" -o $(DIST_DIR)/$(APP_NAME)-darwin-arm64 .

build-linux-amd64:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -ldflags="-s -w" -o $(DIST_DIR)/$(APP_NAME)-linux-amd64 .

build-windows-amd64:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=1 go build -ldflags="-s -w" -o $(DIST_DIR)/$(APP_NAME)-windows-amd64.exe .

# Run the application
run:
	go run .

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -f $(APP_NAME)
	rm -rf $(DIST_DIR)/

# Download dependencies
deps:
	go mod download
	go mod tidy

# Create macOS universal binary
universal-darwin: build-darwin-amd64 build-darwin-arm64
	mkdir -p $(DIST_DIR)
	lipo -create -output $(DIST_DIR)/$(APP_NAME)-darwin-universal $(DIST_DIR)/$(APP_NAME)-darwin-amd64 $(DIST_DIR)/$(APP_NAME)-darwin-arm64

# ==========================================
# macOS App Bundle & DMG Creation
# ==========================================

# Create macOS .app bundle
app-bundle: universal-darwin
	@echo "Creating macOS app bundle..."
	@rm -rf "$(APP_BUNDLE)"
	@mkdir -p "$(APP_BUNDLE)/Contents/MacOS"
	@mkdir -p "$(APP_BUNDLE)/Contents/Resources"
	@cp $(DIST_DIR)/$(APP_NAME)-darwin-universal "$(APP_BUNDLE)/Contents/MacOS/$(APP_NAME)"
	@cp $(BUILD_DIR)/Info.plist "$(APP_BUNDLE)/Contents/"
	@cp $(BUILD_DIR)/icon.icns "$(APP_BUNDLE)/Contents/Resources/"
	@# Update version in Info.plist
	@sed -i '' 's/<string>1.0.0<\/string>/<string>$(VERSION)<\/string>/g' "$(APP_BUNDLE)/Contents/Info.plist" 2>/dev/null || true
	@# Set executable permissions
	@chmod +x "$(APP_BUNDLE)/Contents/MacOS/$(APP_NAME)"
	@echo "App bundle created: $(APP_BUNDLE)"

# Create DMG installer (requires create-dmg: brew install create-dmg)
dmg: app-bundle
	@echo "Creating DMG installer..."
	@chmod +x $(BUILD_DIR)/create-dmg.sh
	@bash $(BUILD_DIR)/create-dmg.sh $(VERSION)

# Full macOS release build
release-macos: clean dmg
	@echo "macOS release build complete!"
	@ls -la $(DIST_DIR)/*.dmg 2>/dev/null || echo "DMG not found"

# Install locally (macOS/Linux)
install: build
	mkdir -p ~/bin
	cp $(APP_NAME) ~/bin/
	@echo "Installed to ~/bin/$(APP_NAME)"

# Install app bundle to /Applications (macOS)
install-app: app-bundle
	@echo "Installing to /Applications..."
	@rm -rf "/Applications/$(APP_DISPLAY_NAME).app"
	@cp -R "$(APP_BUNDLE)" /Applications/
	@echo "Installed: /Applications/$(APP_DISPLAY_NAME).app"
