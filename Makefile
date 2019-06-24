SHELL = /bin/bash
OS = $(shell uname -s)

# Project variables
PACKAGE = github.com/eclipse-iofog/iofog-operator
BINARY_NAME = iofog-operator
IMAGE = iofog/iofog-operator

# Build variables
BUILD_DIR ?= bin
BUILD_PACKAGE = $(PACKAGE)/cmd/manager
VERSION ?= $(shell git rev-parse --abbrev-ref HEAD)
COMMIT_HASH ?= $(shell git rev-parse --short HEAD 2>/dev/null)
BUILD_DATE ?= $(shell date +%FT%T%z)
LDFLAGS += -X main.Version=$(VERSION) -X main.CommitHash=$(COMMIT_HASH) -X main.BuildDate=$(BUILD_DATE)
export CGO_ENABLED ?= 0
ifeq ($(VERBOSE), 1)
	GOARGS += -v
endif

# Golang variables
DEP_VERSION = 0.5.0
GOLANG_VERSION = 1.11
GOFILES_NOVENDOR = $(shell find . -type f -name '*.go' -not -path "./vendor/*" -not -path "./client/*")

BRANCH ?= $(TRAVIS_BRANCH)
RELEASE_TAG ?= 0.0.0


.PHONY: clean
clean: ## Clean the working area and the project
	rm -rf $(BUILD_DIR)/ vendor/

bin/dep: bin/dep-$(DEP_VERSION)
	@ln -sf dep-$(DEP_VERSION) bin/dep
bin/dep-$(DEP_VERSION):
	@mkdir -p bin
	curl https://raw.githubusercontent.com/golang/dep/master/install.sh | INSTALL_DIRECTORY=bin DEP_RELEASE_TAG=v$(DEP_VERSION) sh
	@mv bin/dep $@

.PHONY: vendor
vendor: bin/dep ## Install dependencies
	bin/dep ensure -v -vendor-only

.PHONY: build
build: GOARGS += -tags "$(GOTAGS)" -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)
build:
ifneq ($(IGNORE_GOLANG_VERSION_REQ), 1)
	@printf "$(GOLANG_VERSION)\n$$(go version | awk '{sub(/^go/, "", $$3);print $$3}')" | sort -t '.' -k 1,1 -k 2,2 -k 3,3 -g | head -1 | grep -q -E "^$(GOLANG_VERSION)$$" || (printf "Required Go version is $(GOLANG_VERSION)\nInstalled: `go version`" && exit 1)
endif
	go build $(GOARGS) $(BUILD_PACKAGE)

.PHONY: build-img
build-img:
	docker build --rm -t $(IMAGE):latest -f Dockerfile .

.PHONY: push-img
push-img:
	@echo $(DOCKER_PASS) | docker login -u $(DOCKER_USER) --password-stdin
ifeq ($(BRANCH), master)
	# Master branch
	docker push $(IMAGE):latest
	docker tag $(IMAGE):latest $(IMAGE):$(RELEASE_TAG)
	docker push $(IMAGE):$(RELEASE_TAG)
endif
ifneq (,$(findstring release,$(BRANCH)))
	# Release branch
	docker tag $(IMAGE):latest $(IMAGE):rc-$(RELEASE_TAG)
	docker push $(IMAGE):rc-$(RELEASE_TAG)
else
	# Develop and feature branches
	docker tag $(IMAGE):latest $(IMAGE)-$(BRANCH):latest
	docker push $(IMAGE)-$(BRANCH):latest
	docker tag $(IMAGE):latest $(IMAGE)-$(BRANCH):$(COMMIT_HASH)
	docker push $(IMAGE)-$(BRANCH):$(COMMIT_HASH)
endif

.PHONY: fmt
fmt:
	@gofmt -s -w $(GOFILES_NOVENDOR)

.PHONY: test
test:
	set -o pipefail; go list ./... | xargs -n1 go test $(GOARGS) -v -parallel 1 2>&1 | tee test.txt

# Docker image targets
.PHONY: build-img
build-img: ## Builds docker image for the operator
	docker build --rm -t $(IMAGE):$(TAG) -f build/Dockerfile .

.PHONY: push-img
push-img: ## Pushes the docker image to docker hub
	@echo $(DOCKER_PASS) | docker login -u $(DOCKER_USER) --password-stdin
	docker push $(IMAGE):$(TAG)

# Util targets
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