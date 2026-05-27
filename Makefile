.PHONY: init build test lint cross-compile

BINARY_NAME := sshelob
BUILD_DIR := build
GO ?= go
GOBIN ?= $(shell $(GO) env GOPATH)/bin
GOLANGCI_LINT := $(GOBIN)/golangci-lint
GOLANGCI_LINT_PKG := github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

init:
	$(GO) install $(GOLANGCI_LINT_PKG)

build:
	@mkdir -p $(BUILD_DIR)
	$(GO) build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/sshelob

test:
	$(GO) test ./... -v

lint: init
	$(GOLANGCI_LINT) run ./...

cross-compile:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/sshelob
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/sshelob
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/sshelob
