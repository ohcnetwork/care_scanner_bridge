#!/bin/bash

# Create beautiful DMG installer for Care Scanner Bridge
# Requires: create-dmg (install via: brew install create-dmg)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="${SCRIPT_DIR}/../.."
BUILD_DIR="${PROJECT_ROOT}/dist"
APP_NAME="Care Scanner Bridge"
DMG_NAME="Care-Scanner-Bridge"
VERSION="${1:-1.0.0}"

# Paths
APP_BUNDLE="${BUILD_DIR}/${APP_NAME}.app"
DMG_OUTPUT="${BUILD_DIR}/${DMG_NAME}-${VERSION}.dmg"
DMG_BACKGROUND="${SCRIPT_DIR}/dmg-background.png"
ICON_FILE="${SCRIPT_DIR}/icon.icns"

echo "========================================"
echo "Creating DMG Installer for ${APP_NAME}"
echo "Version: ${VERSION}"
echo "========================================"

# Check for create-dmg
if ! command -v create-dmg &> /dev/null; then
    echo "create-dmg is not installed. Installing via Homebrew..."
    if command -v brew &> /dev/null; then
        brew install create-dmg
    else
        echo "Error: Please install Homebrew first, then run:"
        echo "  brew install create-dmg"
        exit 1
    fi
fi

# Check if app bundle exists
if [ ! -d "${APP_BUNDLE}" ]; then
    echo "Error: App bundle not found at ${APP_BUNDLE}"
    echo "Please run 'make app-bundle' first"
    exit 1
fi

# Remove old DMG if exists
if [ -f "${DMG_OUTPUT}" ]; then
    echo "Removing existing DMG..."
    rm -f "${DMG_OUTPUT}"
fi

# Check for background image
if [ ! -f "${DMG_BACKGROUND}" ]; then
    echo "Warning: Background image not found at ${DMG_BACKGROUND}"
    echo "DMG will be created without custom background"
fi

echo "Creating DMG..."

# Create the DMG with beautiful UI
# Window size: 835x600 (standard for professional DMG installers)
# App icon position: 230, 295 (left side)
# Applications folder position: 593, 295 (right side)
create-dmg \
    --volname "${APP_NAME}" \
    --volicon "${ICON_FILE}" \
    --background "${DMG_BACKGROUND}" \
    --window-pos 200 120 \
    --window-size 835 600 \
    --icon-size 128 \
    --icon "${APP_NAME}.app" 230 295 \
    --hide-extension "${APP_NAME}.app" \
    --app-drop-link 593 295 \
    --no-internet-enable \
    "${DMG_OUTPUT}" \
    "${APP_BUNDLE}" \
    || {
        # create-dmg returns 2 if it can't set icon positions (non-fatal)
        if [ $? -eq 2 ]; then
            echo "Warning: Could not set all icon positions, but DMG was created"
        else
            echo "Error creating DMG"
            exit 1
        fi
    }

echo ""
echo "========================================"
echo "DMG created successfully!"
echo "Output: ${DMG_OUTPUT}"
echo "========================================"

# Optional: Show DMG size
if [ -f "${DMG_OUTPUT}" ]; then
    SIZE=$(du -h "${DMG_OUTPUT}" | cut -f1)
    echo "Size: ${SIZE}"
fi
