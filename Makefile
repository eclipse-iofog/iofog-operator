VERSION = $(shell cat PROJECT | grep "version:" | sed "s/^version: //g")
GOFILES_NOVENDOR = $(shell find . -type f -name '*.go' -not -path "./vendor/*")
PREFIX = github.com/eclipse-iofog/iofog-operator/v2/internal/util
LDFLAGS += -X $(PREFIX).portManagerTag=develop
LDFLAGS += -X $(PREFIX).kubeletTag=develop
LDFLAGS += -X $(PREFIX).proxyTag=develop
LDFLAGS += -X $(PREFIX).routerTag=develop
LDFLAGS += -X $(PREFIX).controllerTag=develop
LDFLAGS += -X $(PREFIX).repo=gcr.io/focal-freedom-236620
GO_SDK_MODULE = iofog-go-sdk/v2@develop

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

# Download modules and vendor them
modules: vendor
	@for module in $(GO_SDK_MODULE); do \
		go get github.com/eclipse-iofog/$$module; \
	done

# Vendor all modules
vendor:
	@go mod vendor

# Build manager binary
.PHONY: build
build: GOARGS += -mod=vendor -ldflags "$(LDFLAGS)"
build: fmt
	go build $(GOARGS) -o bin/iofog-operator main.go

# Install CRDs into a cluster
install: manifests kustomize
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests kustomize
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests kustomize
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
fmt:
	@gofmt -s -w $(GOFILES_NOVENDOR)

## Lint the source
lint: fmt
	@golangci-lint run --timeout 5m0s

# Generate code
generate: gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Gen is not done in Docker to avoid downloading modules. controller-gen does not work with modules
# https://github.com/kubernetes-sigs/controller-tools/issues/327
docker: vendor gen
	docker build -t iofog-operator .

# find or download controller-gen
# download controller-gen if necessary
gen:
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

kustomize:
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

# Generate bundle manifests and metadata, then validate generated files.
.PHONY: bundle
bundle: manifests kustomize
	operator-sdk generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | operator-sdk generate bundle -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)
	operator-sdk bundle validate ./bundle
