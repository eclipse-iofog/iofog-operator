SHELL = /bin/bash
OS = $(shell uname -s)

# Project variables
PACKAGE = github.com/eclipse-iofog/iofog-operator/v2
BINARY_NAME = iofog-operator
IMAGE = iofog/iofog-operator

# Build variables
BUILD_DIR ?= bin
BUILD_PACKAGE = $(PACKAGE)/cmd/manager
export CGO_ENABLED ?= 0
ifeq ($(VERBOSE), 1)
	GOARGS += -v
endif

GOLANG_VERSION = 1.12

GOFILES_NOVENDOR = $(shell find . -type f -name '*.go' -not -path "./vendor/*" -not -path "./client/*")

MAJOR ?= $(shell cat version | grep MAJOR | sed 's/MAJOR=//g')
MINOR ?= $(shell cat version | grep MINOR | sed 's/MINOR=//g')
PATCH ?= $(shell cat version | grep PATCH | sed 's/PATCH=//g')
SUFFIX ?= $(shell cat version | grep SUFFIX | sed 's/SUFFIX=//g')
VERSION = $(MAJOR).$(MINOR).$(PATCH)$(SUFFIX)
PREFIX = github.com/eclipse-iofog/iofog-operator/v2/internal/util
LDFLAGS += -X $(PREFIX).portManagerTag=develop
LDFLAGS += -X $(PREFIX).kubeletTag=develop
LDFLAGS += -X $(PREFIX).proxyTag=develop
LDFLAGS += -X $(PREFIX).routerTag=develop
LDFLAGS += -X $(PREFIX).controllerTag=develop
LDFLAGS += -X $(PREFIX).repo=gcr.io/focal-freedom-236620
GO_SDK_MODULE = iofog-go-sdk/v2@develop

.PHONY: clean
clean: ## Clean the working area and the project
	rm -rf $(BUILD_DIR)/

.PHONY: modules
modules: get vendor ## Get modules and vendor them

.PHONY: get
get: ## Pull modules
	@for module in $(GO_SDK_MODULE); do \
		go get github.com/eclipse-iofog/$$module; \
	done

.PHONY: vendor
vendor: # Vendor all deps
	@go mod vendor
	@for dep in golang.org/x/tools k8s.io/gengo; do \
		git checkout -- vendor/$$dep; \
	done \

.PHONY: build
build: GOARGS += -mod=vendor -tags "$(GOTAGS)" -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)
build: fmt gen
ifneq ($(IGNORE_GOLANG_VERSION_REQ), 1)
	@printf "$(GOLANG_VERSION)\n$$(go version | awk '{sub(/^go/, "", $$3);print $$3}')" | sort -t '.' -k 1,1 -k 2,2 -k 3,3 -g | head -1 | grep -q -E "^$(GOLANG_VERSION)$$" || (printf "Required Go version is $(GOLANG_VERSION)\nInstalled: `go version`" && exit 1)
endif
	go build $(GOARGS) $(BUILD_PACKAGE)

.PHONY: gen
gen: ## Generate code
	GOFLAGS=-mod=vendor deepcopy-gen -i ./pkg/apis/iofog -o . --go-header-file ./vendor/k8s.io/gengo/boilerplate/boilerplate.go.txt

.PHONY: fmt
fmt:
	@gofmt -s -w $(GOFILES_NOVENDOR)

.PHONY: test
test:
	set -o pipefail; go list -mod=vendor ./... | xargs -n1 go test -mod=vendor $(GOARGS) -v -parallel 1 2>&1 | tee test.txt

.PHONY: build-img
build-img:
	docker build -t eclipse-iofog/operator:latest -f build/Dockerfile .

.PHONY: list
list: ## List all make targets
	@$(MAKE) -pRrn : -f $(MAKEFILE_LIST) 2>/dev/null | awk -v RS= -F: '/^# File/,/^# Finished Make data base/ {if ($$1 !~ "^[#.]") {print $$1}}' | egrep -v -e '^[^[:alnum:]]' -e '^$@$$' | sort

.PHONY: help
.DEFAULT_GOAL := help
help:
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

# Variable outputting/exporting rules
var-%: ; @echo $($*)
varexport-%: ; @echo $*=$($*)
