# Development

## Building from Source

Requires Go 1.21+.

```bash
git clone https://github.com/lumipallolabs/diskdive.git
cd diskdive
go build .
go test ./...
```

## macOS App Bundle

Build a native macOS app with DMG installer:

```bash
./scripts/build-mac-app.sh
```

Version is read from the `VERSION` file in the project root. This creates:
- `build/DiskDive.app` - Universal binary (Intel + Apple Silicon)
- `build/DiskDive-X.Y.Z.dmg` - DMG installer

### Signed & Notarized Builds

For distribution, sign and notarize the app:

1. **One-time setup:**
   ```bash
   # Store notarization credentials in keychain
   xcrun notarytool store-credentials notarytool \
     --apple-id your@email.com \
     --team-id YOURTEAMID

   # Find your signing identity
   security find-identity -v -p codesigning
   ```

2. **Create `.env` file** (gitignored):
   ```bash
   export SIGN_IDENTITY="<certificate hash>"
   export NOTARIZE_PROFILE="notarytool"
   ```

3. **Build:**
   ```bash
   source .env && ./scripts/build-mac-app.sh
   ```

## Architecture

DiskDive separates core business logic from the UI:

```
internal/
  core/       # Pure business logic (scanning, state, events)
  ui/tui/     # Terminal UI (Bubble Tea)
  scanner/    # Filesystem scanning
  model/      # Data structures
  watcher/    # Filesystem monitoring
```

This makes it possible to build alternative frontends (GUI, web) using the same core.

## Contributing

Contributions are welcome! See [CLAUDE.md](CLAUDE.md) for code style and guidelines.
