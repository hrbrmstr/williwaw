build:
  rm -f gofir
  go build .

build-win:
  rm -f gofir.exe
  GOOS=windows GOARCH=amd64 go build -o bin/gofir.exe .

update-modules:
  go get -u ./...
  go mod tidy
  go build .

run: build
  SEEKRIT_TOKEN=bye DB_PATH=readings.db ./gofir
