SHELL:=/usr/bin/env bash
VERSION:=$(shell [ -z "$$(git tag --points-at HEAD)" ] && echo "$$(git describe --always --long --dirty | sed 's/^v//')" || echo "$$(git tag --points-at HEAD | sed 's/^v//')")
GO_FILES:=$(shell find . -name '*.go' | grep -v /vendor/)
BIN_NAME:=wxcal

default: help

# via https://marmelab.com/blog/2016/02/29/auto-documented-makefile.html
.PHONY: help
help:  ## Print help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: all
all: clean build build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64  ## Build for the current platform and: Linux/amd64, Linux/arm64, macOS/amd64, macOS/arm64

.PHONY: clean
clean:  ## Remove built products in ./out
	rm -rf ./out

.PHONY: lint
lint:  ## Lint all .go files
	@for file in ${GO_FILES} ;  do \
		echo "$$file" ; \
		golint $$file ; \
	done

.PHONY: build
build: lint  ## Build for the current platform & architecture to ./out
	mkdir -p out
	go build -ldflags="-X main.ProductVersion=${VERSION}" -o ./out/${BIN_NAME} .

.PHONY: build-linux-amd64
build-linux-amd64: lint  ## Build for Linux/amd64 to ./out/linux-amd64
	mkdir -p out/linux-amd64
	env GOOS=linux GOARCH=amd64 go build -ldflags="-X main.ProductVersion=${VERSION}" -o ./out/linux-amd64/${BIN_NAME} .

.PHONY: build-linux-arm64
build-linux-arm64: lint  ## Build for Linux/arm64 to ./out/linux-arm64
	mkdir -p out/linux-arm64
	env GOOS=linux GOARCH=arm64 go build -ldflags="-X main.ProductVersion=${VERSION}" -o ./out/linux-arm64/${BIN_NAME} .

.PHONY: build-darwin-amd64
build-darwin-amd64: lint  ## Build for macOS (Darwin) / amd64 to ./out/darwin-amd64
	mkdir -p out/darwin-amd64
	env GOOS=darwin GOARCH=amd64 go build -ldflags="-X main.ProductVersion=${VERSION}" -o ./out/darwin-amd64/${BIN_NAME} .

.PHONY: build-darwin-arm64
build-darwin-arm64: lint  ## Build for macOS (Darwin) / arm64 to ./out/darwin-arm64
	mkdir -p out/darwin-arm64
	env GOOS=darwin GOARCH=arm64 go build -ldflags="-X main.ProductVersion=${VERSION}" -o ./out/darwin-arm64/${BIN_NAME} .

.PHONY: install
install: lint  ## Build & install wxcal directly to /usr/local/bin
	go build -ldflags="-X main.ProductVersion=${VERSION}" -o /usr/local/bin/${BIN_NAME} .

.PHONY: uninstall
uninstall:  ## Remove wxcal from /usr/local/bin
	rm /usr/local/bin/${BIN_NAME}

.PHONY: cdzombak-deploy-burr
cdzombak-deploy-burr: clean build-linux-amd64
	scp ./out/linux-amd64/${BIN_NAME} burr:~/wxcal
