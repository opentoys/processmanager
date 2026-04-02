BINARY_NAME=pm
BUILD_DIR=.

GIT_TAG=$(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
VERSION=$(shell date +%Y.%m.%d-%H%M)
GO_VERSION=$(shell go version | awk '{print $$3}')
FULL_VERSION=$(GIT_TAG)($(VERSION))
LDFLAGS=-s -w \
	-X processmanager/internal/utils.Version=$(FULL_VERSION) \
	-X processmanager/internal/utils.GoVersion=$(GO_VERSION)

.PHONY: build clean

build:
	go build -trimpath -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd

clean:
	rm -f $(BUILD_DIR)/$(BINARY_NAME)