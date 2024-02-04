# use .env here
set dotenv-load := true

binary := "williwaw"

# list the available recipes
@default:
  just --list

# build the core macOS binary since that's where I develop from
build:
  GOOS=darwin GOARCH=arm64 go build -o bin/aarch64-{{binary}} .
  GOOS=darwin GOARCH=amd64 go build -o bin/x86_64-{{binary}} .
  lipo -create -output bin/{{binary}} bin/aarch64-{{binary}} bin/x86_64-{{binary}}
  codesign --force --verify --verbose --sign "${APPLE_DEV_ID}" "bin/{{binary}}"

# build the Windows binary (since I'm copying it to the tablet)
build-win:
  GOOS=windows GOARCH=amd64 go build -o bin/{{binary}}.exe .

# {fir} is under active development
update-modules:
  go get -u ./...
  go mod tidy

# run for testing
run:
  scp ceres:/Volumes/crucial/tempest/2024-02-03.db ./readings.db
  go run .

# SBOM
sbom:
  syft packages dir:. -o json > sbom.json