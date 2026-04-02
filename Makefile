BINARY_NAME=pm
BUILD_DIR=.

VERSION=$(shell date +%Y.%m.%d-%H%M)
GO_VERSION=$(shell go version | awk '{print $$3}')
LDFLAGS=-s -w \
	-X processmanager/internal/utils.Version=$(VERSION) \
	-X processmanager/internal/utils.GoVersion=$(GO_VERSION)

.PHONY: build clean

build:
	go build -trimpath -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd

clean:
	rm -f $(BUILD_DIR)/$(BINARY_NAME)

