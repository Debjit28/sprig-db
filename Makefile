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
	@echo "🧪 Running Unit Tests..."
	@go test -v ./...
	@echo "\n🚀 Running Benchmarks to test Storage Performance..."
	@go test -bench . -benchmem ./...

loadtest:
	@echo "🔥 Compiling and Running Massive Concurrent HTTP Load Test..."
	@go run ./cmd/loadtest/main.go

