# List the available development commands.
default:
    @just --list

# Install the locked TypeScript and Dart dependencies without global packages.
# The Go SDK uses only the standard library.
install:
    npm ci
    fvm dart pub get

# Build and check every SDK, then verify cross-language exercise compatibility.
check:
    npm run check
    fvm dart analyze
    fvm dart format --output=none --set-exit-if-changed lib test tool
    fvm dart test
    test -z "$(gofmt -l exercise tool)" || (gofmt -l exercise tool; exit 1)
    go vet ./exercise ./tool
    go test ./exercise
    npm run parity

# Run the shared fixtures through every SDK test suite.
test:
    npm test
    fvm dart test
    go test ./exercise

# Fuzz the Go decoder for a while (default 1 minute).
fuzz time="1m":
    go test -run '^$' -fuzz FuzzDecode -fuzztime {{time}} ./exercise

# Build the TypeScript Git dependency.
build:
    npm run build

# Check the contents of the npm package without publishing it.
pack-check:
    npm pack --dry-run
