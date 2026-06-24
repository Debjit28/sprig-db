# Variables
BINARY_NAME=sprig-db
CMD_DIR=./cmd
BUILD_DIR=./bin


# Build the binary
build:
	@go build -o ./bin/sprig-db ./cmd/main.go

run: build
	@./bin/sprig-db

test: 
	@go test -v ./...
