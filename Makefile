.PHONY: all build build-web build-go test bench clean docker-build

BINARY_NAME=streamgate
BENCH_NAME=streamgate-bench
BIN_DIR=bin

all: test build

build: build-web build-go

build-web:
	@echo "Building React 19 web console..."
	cd web && npm ci && npm run build

build-go:
	@echo "Compiling streamgate daemon..."
	go build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/streamgate
	@echo "Compiling streamgate-bench..."
	go build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/$(BENCH_NAME) ./cmd/streamgate-bench

test:
	@echo "Running unit and race detector test suites..."
	go test -race -v ./...

bench: build-go
	@echo "Starting benchmark run..."
	./$(BIN_DIR)/$(BENCH_NAME) -clients 1000 -rate 50 -duration 10s

docker-build:
	docker build -t streamgate:latest .

clean:
	rm -rf $(BIN_DIR) web/dist web/node_modules
