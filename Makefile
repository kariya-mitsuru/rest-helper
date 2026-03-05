# SPDX-License-Identifier: MIT

VERSION ?= $(shell cat VERSION 2>/dev/null | tr -d '[:space:]')
LDFLAGS := -X main.version=v$(VERSION)
BINARY  := rest-helper

.DEFAULT_GOAL := help

.PHONY: build clean release help

build:          ## Build the binary
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

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
