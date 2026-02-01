#!/bin/bash
set -e

# Configuration
APP_NAME="DiskDive"
BINARY_NAME="diskdive"
ICON_PNG="assets/icon.png"
BUNDLE_ID="com.lumipallolabs.diskdive"

# Read version from VERSION file (single source of truth)
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
if [ -f "$PROJECT_ROOT/VERSION" ]; then
    VERSION="$(cat "$PROJECT_ROOT/VERSION" | tr -d '[:space:]')"
else
    echo "Error: VERSION file not found"
    exit 1
fi

# Signing configuration (set these environment variables to enable signing)
# SIGN_IDENTITY="Developer ID Application: Your Name (TEAMID)"
# APPLE_ID="your@email.com"
# TEAM_ID="YOURTEAMID"
# NOTARIZE_PASSWORD="@keychain:notarytool" or app-specific password

# Paths (SCRIPT_DIR and PROJECT_ROOT already set above)
BUILD_DIR="$PROJECT_ROOT/build"
APP_BUNDLE="$BUILD_DIR/$APP_NAME.app"
DMG_NAME="$APP_NAME-$VERSION.dmg"

cd "$PROJECT_ROOT"

# Check for icon
if [ ! -f "$ICON_PNG" ]; then
    echo "Error: $ICON_PNG not found in project root"
    echo "Please add a 1024x1024 PNG icon named icon.png"
    exit 1
fi

echo "==> Building $APP_NAME v$VERSION..."

# Clean previous build
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"

# Build the Go binary (universal binary for Intel + Apple Silicon)
# CGO is required for fsevents on macOS
# Version is injected via ldflags
echo "==> Compiling universal binary..."
LDFLAGS="-X main.Version=$VERSION"
CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -ldflags "$LDFLAGS" -o "$BUILD_DIR/${BINARY_NAME}-amd64" .
CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -ldflags "$LDFLAGS" -o "$BUILD_DIR/${BINARY_NAME}-arm64" .
lipo -create -output "$BUILD_DIR/$BINARY_NAME" "$BUILD_DIR/${BINARY_NAME}-amd64" "$BUILD_DIR/${BINARY_NAME}-arm64"
rm "$BUILD_DIR/${BINARY_NAME}-amd64" "$BUILD_DIR/${BINARY_NAME}-arm64"

# Create .icns from PNG
echo "==> Creating icon..."
ICONSET="$BUILD_DIR/icon.iconset"
mkdir -p "$ICONSET"

sips -z 16 16     "$ICON_PNG" --out "$ICONSET/icon_16x16.png"      2>/dev/null
sips -z 32 32     "$ICON_PNG" --out "$ICONSET/icon_16x16@2x.png"   2>/dev/null
sips -z 32 32     "$ICON_PNG" --out "$ICONSET/icon_32x32.png"      2>/dev/null
sips -z 64 64     "$ICON_PNG" --out "$ICONSET/icon_32x32@2x.png"   2>/dev/null
sips -z 128 128   "$ICON_PNG" --out "$ICONSET/icon_128x128.png"    2>/dev/null
sips -z 256 256   "$ICON_PNG" --out "$ICONSET/icon_128x128@2x.png" 2>/dev/null
sips -z 256 256   "$ICON_PNG" --out "$ICONSET/icon_256x256.png"    2>/dev/null
sips -z 512 512   "$ICON_PNG" --out "$ICONSET/icon_256x256@2x.png" 2>/dev/null
sips -z 512 512   "$ICON_PNG" --out "$ICONSET/icon_512x512.png"    2>/dev/null
sips -z 1024 1024 "$ICON_PNG" --out "$ICONSET/icon_512x512@2x.png" 2>/dev/null

iconutil -c icns "$ICONSET" -o "$BUILD_DIR/AppIcon.icns"
rm -rf "$ICONSET"

# Create .app bundle structure
echo "==> Creating app bundle..."
mkdir -p "$APP_BUNDLE/Contents/MacOS"
mkdir -p "$APP_BUNDLE/Contents/Resources"

# Copy binary
cp "$BUILD_DIR/$BINARY_NAME" "$APP_BUNDLE/Contents/MacOS/"

# Copy icon
cp "$BUILD_DIR/AppIcon.icns" "$APP_BUNDLE/Contents/Resources/"

# Create launcher script (opens Terminal and runs the CLI)
cat > "$APP_BUNDLE/Contents/MacOS/launcher" << 'LAUNCHER'
#!/bin/bash
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
osascript <<EOF
tell application "Terminal"
    activate
    do script "cd ~ && \"$SCRIPT_DIR/diskdive\""
end tell
EOF
LAUNCHER
chmod +x "$APP_BUNDLE/Contents/MacOS/launcher"

# Create Info.plist
cat > "$APP_BUNDLE/Contents/Info.plist" << PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleName</key>
    <string>$APP_NAME</string>
    <key>CFBundleDisplayName</key>
    <string>$APP_NAME</string>
    <key>CFBundleIdentifier</key>
    <string>$BUNDLE_ID</string>
    <key>CFBundleVersion</key>
    <string>$VERSION</string>
    <key>CFBundleShortVersionString</key>
    <string>$VERSION</string>
    <key>CFBundleExecutable</key>
    <string>launcher</string>
    <key>CFBundleIconFile</key>
    <string>AppIcon</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>LSMinimumSystemVersion</key>
    <string>12.0</string>
    <key>NSHighResolutionCapable</key>
    <true/>
</dict>
</plist>
PLIST

# Code signing
if [ -n "$SIGN_IDENTITY" ]; then
    echo "==> Signing app bundle..."

    # Sign the binary first
    codesign --force --options runtime --timestamp \
        --sign "$SIGN_IDENTITY" \
        "$APP_BUNDLE/Contents/MacOS/$BINARY_NAME"

    # Sign the launcher script
    codesign --force --options runtime --timestamp \
        --sign "$SIGN_IDENTITY" \
        "$APP_BUNDLE/Contents/MacOS/launcher"

    # Sign the entire app bundle
    codesign --force --options runtime --timestamp \
        --sign "$SIGN_IDENTITY" \
        "$APP_BUNDLE"

    echo "==> Verifying signature..."
    codesign --verify --verbose "$APP_BUNDLE"
else
    echo "==> Skipping signing (set SIGN_IDENTITY to enable)"
fi

# Create DMG
echo "==> Creating DMG..."
DMG_TEMP="$BUILD_DIR/dmg_temp"
mkdir -p "$DMG_TEMP"

# Copy app to temp folder
cp -R "$APP_BUNDLE" "$DMG_TEMP/"

# Create Applications symlink
ln -s /Applications "$DMG_TEMP/Applications"

# Create the DMG
hdiutil create -volname "$APP_NAME" \
    -srcfolder "$DMG_TEMP" \
    -ov -format UDZO \
    "$BUILD_DIR/$DMG_NAME"

# Sign the DMG if signing is enabled
if [ -n "$SIGN_IDENTITY" ]; then
    echo "==> Signing DMG..."
    codesign --force --timestamp \
        --sign "$SIGN_IDENTITY" \
        "$BUILD_DIR/$DMG_NAME"
fi

# Notarization
if [ -n "$SIGN_IDENTITY" ] && [ -n "$NOTARIZE_PROFILE" ]; then
    echo "==> Submitting for notarization..."
    xcrun notarytool submit "$BUILD_DIR/$DMG_NAME" \
        --keychain-profile "$NOTARIZE_PROFILE" \
        --wait

    echo "==> Stapling notarization ticket..."
    xcrun stapler staple "$BUILD_DIR/$DMG_NAME"

    echo "==> Verifying notarization..."
    spctl --assess --type open --context context:primary-signature -v "$BUILD_DIR/$DMG_NAME"
else
    if [ -n "$SIGN_IDENTITY" ]; then
        echo ""
        echo "==> Skipping notarization (set NOTARIZE_PROFILE to enable)"
    fi
fi

# Cleanup
rm -rf "$DMG_TEMP"
rm -f "$BUILD_DIR/$BINARY_NAME"
rm -f "$BUILD_DIR/AppIcon.icns"

echo ""
echo "==> Build complete!"
echo "    Version: $VERSION (from VERSION file)"
echo "    App bundle: $APP_BUNDLE"
echo "    DMG: $BUILD_DIR/$DMG_NAME"
echo ""
echo "To change version, edit the VERSION file in the project root."
echo ""
echo "For signed builds, set these environment variables (or use .env file):"
echo "  SIGN_IDENTITY=\"<certificate hash or name>\""
echo "  NOTARIZE_PROFILE=\"notarytool\"  # keychain profile name"
echo ""
echo "To set up signing (one-time):"
echo "  1. Create Developer ID Application cert at developer.apple.com"
echo "  2. Store notarization credentials:"
echo "     xcrun notarytool store-credentials notarytool \\"
echo "       --apple-id your@email.com \\"
echo "       --team-id YOURTEAMID"
