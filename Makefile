# SPDX-License-Identifier: MIT

VERSION ?= $(shell cat VERSION 2>/dev/null | tr -d '[:space:]')
LDFLAGS := -X main.version=v$(VERSION)
BINARY  := rest-helper

.DEFAULT_GOAL := help

.PHONY: build clean release fmt lint help

build:          ## Build the binary
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

fmt:            ## Run go fmt, goimports, and golangci-lint --fix on all Go files
	go fmt ./...
	find . -name '*.go' -exec goimports -w {} +
	golangci-lint run --fix ./...

lint:           ## Check formatting and run linter (no modifications)
	@STATUS=0; \
	FMT=$$(gofmt -l .); \
	if [ -n "$$FMT" ]; then printf "gofmt:\n%s\n" "$$FMT"; STATUS=1; fi; \
	IMP=$$(find . -name '*.go' -exec goimports -l {} +); \
	if [ -n "$$IMP" ]; then printf "goimports:\n%s\n" "$$IMP"; STATUS=1; fi; \
	golangci-lint run ./... || STATUS=1; \
	exit $$STATUS

clean:          ## Remove the binary
	rm -f $(BINARY)

release:        ## Bump version and open a pull request to main (VERSION=x.y.z required)
	@[ -n "$(VERSION)" ] || { printf "Error: VERSION is required.\nUsage: make release VERSION=x.y.z\n"; exit 1; }
	@printf '%s\n' "$(VERSION)" > VERSION
	git add VERSION
	git commit -m "chore: bump version to $(VERSION)"
	git push
	gh pr create --title "chore: bump version to $(VERSION)" --body "" --base main

help:           ## Show this help
	@grep -E '^[a-zA-Z_-]+:[ \t]+##' $(MAKEFILE_LIST) | \
	 awk 'BEGIN {FS = ":[ \t]+## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'
