binary := "williwaw"

# list the available recipes
@default:
  just --list

build:
  GOOS=darwin GOARCH=arm64 go build -o bin/aarch64-{{binary}} .
  GOOS=darwin GOARCH=amd64 go build -o bin/x86_64-{{binary}} .
  lipo -create -output bin/{{binary}} bin/aarch64-{{binary}} bin/x86_64-{{binary}}
  codesign --force --verify --verbose --sign "${APPLE_DEV_ID}" "bin/{{binary}}"

build-win:
  GOOS=windows GOARCH=amd64 go build -o bin/{{binary}}.exe .

update-modules:
  go get -u ./...
  go mod tidy
  go build .

run:
  SEEKRIT_TOKEN=bye DB_PATH=readings.db go run .
