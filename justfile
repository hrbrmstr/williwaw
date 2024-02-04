build:
  rm -f williwaw
  go build .-o bin/williwaw

build-win:
  rm -f williwaw.exe
  GOOS=windows GOARCH=amd64 go build -o bin/williwaw.exe .

update-modules:
  go get -u ./...
  go mod tidy
  go build .

run: build
  SEEKRIT_TOKEN=bye DB_PATH=readings.db ./williwaw
