BINARY_NAME=keyforge
BUILD_DIR=bin
VERSION ?= dev
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

.PHONY: build run test clean release

build:
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/keyforge

run: build
	./$(BUILD_DIR)/$(BINARY_NAME) serve

test:
	go test ./...

clean:
	rm -rf $(BUILD_DIR)

release: clean
	GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64   ./cmd/keyforge
	GOOS=linux   GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64   ./cmd/keyforge
	GOOS=linux   GOARCH=arm   go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm     ./cmd/keyforge
	GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64  ./cmd/keyforge
	GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64  ./cmd/keyforge
	GOOS=android GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-android-arm64 ./cmd/keyforge
