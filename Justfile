# List the available development commands.
default:
    @just --list

# Install the locked TypeScript and Dart dependencies without global packages.
install:
    npm ci
    fvm dart pub get

# Build and check both SDKs, then verify cross-language exercise compatibility.
check:
    npm run check
    fvm dart analyze
    fvm dart format --output=none --set-exit-if-changed lib test tool
    fvm dart test
    npm run parity

# Run the shared fixtures through both SDK test suites.
test:
    npm test
    fvm dart test

# Build the TypeScript Git dependency.
build:
    npm run build

# Check the contents of the npm package without publishing it.
pack-check:
    npm pack --dry-run
