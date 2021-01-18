VERSION = $(shell cat PROJECT | grep "version:" | sed "s/^version: //g")
GOFILES_NOVENDOR = $(shell find . -type f -name '*.go' -not -path "./vendor/*")
PREFIX = github.com/eclipse-iofog/iofog-operator/v2/internal/util
LDFLAGS += -X $(PREFIX).portManagerTag=develop
LDFLAGS += -X $(PREFIX).kubeletTag=develop
LDFLAGS += -X $(PREFIX).proxyTag=develop
LDFLAGS += -X $(PREFIX).routerTag=develop
LDFLAGS += -X $(PREFIX).controllerTag=develop
LDFLAGS += -X $(PREFIX).repo=gcr.io/focal-freedom-236620
GO_SDK_MODULE = iofog-go-sdk/v2@6a3c7233ad91843457c699dc610cc0ff33a73a3c

export CGO_ENABLED ?= 0

# Image URL to use all building/pushing image targets
IMG ?= operator:latest
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: build

modules: vendor ## Download modules and vendor them
	@for module in $(GO_SDK_MODULE); do \
		go get github.com/eclipse-iofog/$$module; \
	done

vendor: ## Vendor all modules
	@go mod vendor

.PHONY: build
build: GOARGS += -mod=vendor -ldflags "$(LDFLAGS)"
build: fmt gen ## Build operator binary
	go build $(GOARGS) -o bin/iofog-operator main.go

install: manifests kustomize ## Install CRDs into a cluster
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

uninstall: manifests kustomize ## Uninstall CRDs from a cluster
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

deploy: manifests kustomize ## Deploy controller in the configured Kubernetes cluster in ~/.kube/config
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

manifests: export GOFLAGS=-mod=vendor
manifests: gen ## Generate manifests e.g. CRD, RBAC etc.
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

fmt: ## Run go fmt against code
	@gofmt -s -w $(GOFILES_NOVENDOR)
 
lint: fmt ## Lint the source
	@golangci-lint run --timeout 5m0s

gen: export GOFLAGS=-mod=vendor
gen: controller-gen ## Generate code using controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

docker:
	docker build -t $(IMG) .

controller-gen: ## Install controller-gen
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.3.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

kustomize: ## Install kustomize
ifeq (, $(shell which kustomize))
	@{ \
	set -e ;\
	KUSTOMIZE_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$KUSTOMIZE_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/kustomize/kustomize/v3@v3.5.4 ;\
	rm -rf $$KUSTOMIZE_GEN_TMP_DIR ;\
	}
KUSTOMIZE=$(GOBIN)/kustomize
else
KUSTOMIZE=$(shell which kustomize)
endif

.PHONY: bundle
bundle: manifests kustomize ## Generate bundle manifests and metadata, then validate generated files.
	operator-sdk generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | operator-sdk generate bundle -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)
	operator-sdk bundle validate ./bundle

test: ## Run unit tests
	set -o pipefail; go list -mod=vendor ./... | xargs -n1 go test -mod=vendor $(GOARGS) -v -parallel 1 2>&1 | tee test.txt

help:
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
