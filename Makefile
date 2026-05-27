.PHONY: build test lint cross-compile

BINARY_NAME := sshelob
BUILD_DIR := build
GO ?= go

build:
	@mkdir -p $(BUILD_DIR)
	$(GO) build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/sshelob

test:
	$(GO) test ./... -v

lint:
	golangci-lint run ./...

cross-compile:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/sshelob
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/sshelob
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/sshelob
