OS = $(shell uname -s | tr '[:upper:]' '[:lower:]')

VERSION = $(shell grep "^version:" PROJECT | head -1 | sed 's/^version: *//' | tr -d '"' | tr -d ' ')
PREFIX = github.com/datasance/iofog-operator/v3/internal/util
LDFLAGS += -X $(PREFIX).routerTag=3.7.0
LDFLAGS += -X $(PREFIX).controllerTag=3.7.3
LDFLAGS += -X $(PREFIX).natsTag=2.12.4
LDFLAGS += -X $(PREFIX).repo=ghcr.io/datasance

export CGO_ENABLED ?= 0
ifeq (${DEBUG},)
else
GOARGS=-gcflags="all=-N -l"
endif

# Image URL to use all building/pushing image targets
REGISTRY ?= ghcr.io/datasance
VERSION_TAG ?= 3.7.2
IMG ?= $(REGISTRY)/operator:$(VERSION_TAG)
BUNDLE_IMG ?= $(REGISTRY)/operator-bundle:$(VERSION_TAG)
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:crdVersions=v1,allowDangerousTypes=true"

# Local testing (no image build)
KUBECONFIG ?= $(HOME)/.kube/config
TEST_NAMESPACE ?= iofog-test
CR_PATH ?= config/cr/
KUBECTL ?= kubectl
export KUBECONFIG

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Check if GOBIN is an absolute path
ifneq ($(shell [ -d $(GOBIN) ] && echo yes),yes)
$(error GOBIN must be set to an absolute path and exist)
endif

all: build

.PHONY: build
build: GOARGS += -ldflags "$(LDFLAGS)"
build: fmt gen ## Build operator binary
	GOARCH=$(GOARCH) GOOS=$(GOOS) go build $(GOARGS) -o bin/iofog-operator main.go

install: manifests kustomize ## Install CRDs into a cluster
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply -f -

uninstall: manifests kustomize ## Uninstall CRDs from a cluster
	$(KUSTOMIZE) build config/crd | $(KUBECTL) delete -f -

deploy: manifests kustomize ## Deploy controller in the configured Kubernetes cluster in ~/.kube/config
	cd config/operator && $(KUSTOMIZE) edit set image ghcr.io/datasance/operator=$(IMG)
	$(KUSTOMIZE) build config/default | $(KUBECTL) apply -f -

.PHONY: local-prep
local-prep: install build ## Install CRDs and build operator for local testing (uses KUBECONFIG)

.PHONY: create-namespace
create-namespace: ## Create TEST_NAMESPACE if it does not exist (uses KUBECONFIG)
	$(KUBECTL) get namespace $(TEST_NAMESPACE) >/dev/null 2>&1 || $(KUBECTL) create namespace $(TEST_NAMESPACE)

.PHONY: deploy-cr
deploy-cr: create-namespace ## Deploy CR(s) from CR_PATH into TEST_NAMESPACE (uses KUBECONFIG)
	$(KUSTOMIZE) build $(CR_PATH) | $(KUBECTL) apply -f - -n $(TEST_NAMESPACE)

.PHONY: run
run: build ## Run operator locally (uses KUBECONFIG, optional WATCH_NAMESPACE=TEST_NAMESPACE)
	WATCH_NAMESPACE=$(TEST_NAMESPACE) ./bin/iofog-operator

.PHONY: test-local
test-local: local-prep deploy-cr ## Install CRDs, build operator, deploy CR; then run: make run
	@echo "Run the operator in another terminal: make run"

manifests: gen ## Generate manifests e.g. CRD, RBAC etc.
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

fmt: ## Run gofmt against code
	@gofmt -s -w .

lint: golangci-lint fmt ## Lint the source
	@$(GOLANGCI_LINT) run --timeout 5m0s

gen: controller-gen ## Generate code using controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

docker:
	docker build -t $(REGISTRY)/$(IMG) .

unit: ## Run unit tests
	set -o pipefail; go list ./... | xargs -n1 go test  $(GOARGS) -v -parallel 1 2>&1 | tee test.txt

feature: bats kubectl kustomize ## Run feature tests
	test/run.bash

bats: ## Install bats
ifeq (, $(shell which bats))
	@{ \
	set -e ;\
	BATS_TMP_DIR=$$(mktemp -d) ;\
	cd $$BATS_TMP_DIR ;\
	git clone https://github.com/bats-core/bats-core.git ;\
	cd bats-core ;\
	git checkout tags/v1.1.0 ;\
	./install.sh /usr/local ;\
	rm -rf $$BATS_TMP_DIR ;\
	}
endif

kubectl: ## Install kubectl
ifeq (, $(shell which kubectl))
	@{ \
	set -e ;\
	KCTL_TMP_DIR=$$(mktemp -d) ;\
	cd $$KCTL_TMP_DIR ;\
	curl -Lo kubectl https://storage.googleapis.com/kubernetes-release/release/v1.25.2/bin/"$(OS)"/amd64/kubectl ;\
	chmod +x kubectl ;\
	mv kubectl /usr/local/bin/ ;\
	rm -rf $$KCTL_TMP_DIR ;\
	}
endif

golangci-lint: ## Install golangci
ifeq (, $(shell which golangci-lint))
	@{ \
	set -e ;\
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.62.2 ;\
	}
GOLANGCI_LINT=$(GOBIN)/golangci-lint
else
GOLANGCI_LINT=$(shell which golangci-lint)
endif

controller-gen: ## Install controller-gen
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.14.0 ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

kustomize: ## Install kustomize
ifeq (, $(shell which kustomize))
	@{ \
	set -e ;\
	go install sigs.k8s.io/kustomize/kustomize/v5@v5.5.0 ;\
	}
KUSTOMIZE=$(GOBIN)/kustomize
else
KUSTOMIZE=$(shell which kustomize)
endif

.PHONY: bundle
bundle: manifests kustomize ## Generate bundle manifests and metadata, then validate generated files.
	operator-sdk generate kustomize manifests -q
	cd config/operator && $(KUSTOMIZE) edit set image ghcr.io/datasance/operator=$(IMG)
	$(KUSTOMIZE) build config/manifests | operator-sdk generate bundle -q --overwrite --version $(VERSION_TAG) $(BUNDLE_METADATA_OPTS)
	operator-sdk bundle validate ./bundle


.PHONY: bundle-build
bundle-build: ## Build the bundle image.
	docker buildx build --platform=linux/amd64 -f bundle.Dockerfile -t $(REGISTRY)/$(BUNDLE_IMG) .

help:
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
