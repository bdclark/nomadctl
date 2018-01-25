SHELL := /bin/bash

# The name of the executable (default is current directory name)
TARGET := $(shell echo $${PWD\#\#*/})
.DEFAULT_GOAL: $(TARGET)


GIT_COMMIT := $(shell git rev-parse HEAD)
GIT_DIRTY := $(if $(shell git status --porcelain),+CHANGES)

# Use linker flags to provide version/build settings to the target
LDFLAGS=-ldflags "-X=github.com/bdclark/nomadctl/version.GitCommit=$(GIT_COMMIT)$(GIT_DIRTY)"

# go source files, ignore vendor directory
SRC = $(shell find . -type f -name '*.go' -not -path "./vendor/*")

.PHONY: all build clean install uninstall fmt simplify check run

all: check install

$(TARGET): $(SRC)
	@echo "==> Running $@..."
	@go build $(LDFLAGS) -o $(TARGET)

build: $(TARGET)
	@true

clean:
	@echo "==> Running $@..."
	@rm -f $(TARGET)

install:
	@echo "==> Running $@..."
	@go install $(LDFLAGS)

uninstall: clean
	@echo "==> Running $@..."
	@rm -f $$(which ${TARGET})

fmt:
	@echo "==> Running $@..."
	@gofmt -l -w $(SRC)

simplify:
	@echo "==> Running $@..."
	@gofmt -s -l -w $(SRC)

check:
	@echo "==> Running $@..."
	@test -z $(shell gofmt -l main.go | tee /dev/stderr) || echo "[WARN] Fix formatting issues with 'make fmt'"
	@for d in $$(go list ./... | grep -v /vendor/); do golint $${d}; done
	@go tool vet $(SRC)

run: install
	@$(TARGET)
