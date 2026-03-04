BINARY_NAME=keyforge
BUILD_DIR=bin

.PHONY: build run test clean

build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/keyforge

run: build
	./$(BUILD_DIR)/$(BINARY_NAME) serve

test:
	go test ./...

clean:
	rm -rf $(BUILD_DIR)
