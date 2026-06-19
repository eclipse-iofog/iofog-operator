OS = $(shell uname -s | tr '[:upper:]' '[:lower:]')

GOBIN ?= $(shell go env GOBIN)
ifeq ($(GOBIN),)
GOBIN := $(shell go env GOPATH)/bin
endif

export PATH := $(GOBIN):$(PATH)

# golangci-lint — pinned version; override with GOLANGCI_LINT_VERSION=vX.Y.Z
GOLANGCI_LINT_VERSION ?= v2.12.2
GOLANGCI_LINT         := $(GOBIN)/golangci-lint

# Security tooling — gosec runs outside golangci-lint (edgelet / go-sdk pattern)
GOVULNCHECK_VERSION ?= v1.1.4
GOSEC_VERSION       ?= v2.22.2
GOSEC_SCOPE         := ./...

VERSION = $(shell grep "^version:" PROJECT | head -1 | sed 's/^version: *//' | tr -d '"' | tr -d ' ')
PREFIX = github.com/eclipse-iofog/iofog-operator/v3/internal/util

# Canonical CRD group in Go source (see apis/*/v3/groupversion_info.go).
SOURCE_CRD_GROUP = datasance.com

# Dual-mirror flavor (override in CI — see RFC R6–R12)
OPERATOR_CRD_GROUP ?= iofog.org
OPERATOR_COMPONENT_LABEL_DOMAIN ?= iofog.org
IMAGE_REGISTRY ?= ghcr.io/eclipse-iofog

# Default component image tags (RFC R10: ghcr.io/eclipse-iofog/*:3.8.0)
CONTROLLER_IMAGE_TAG ?= 3.8.0-rc.1
ROUTER_IMAGE_TAG ?= 3.8.0-rc.1
NATS_IMAGE_TAG ?= 3.8.0-rc.1

LDFLAGS += -X $(PREFIX).routerTag=$(ROUTER_IMAGE_TAG)
LDFLAGS += -X $(PREFIX).controllerTag=$(CONTROLLER_IMAGE_TAG)
LDFLAGS += -X $(PREFIX).natsTag=$(NATS_IMAGE_TAG)
LDFLAGS += -X $(PREFIX).repo=$(IMAGE_REGISTRY)
LDFLAGS += -X $(PREFIX).componentLabelDomain=$(OPERATOR_COMPONENT_LABEL_DOMAIN)

export CGO_ENABLED ?= 0
ifeq (${DEBUG},)
else
GOARGS=-gcflags="all=-N -l"
endif

# Image URL to use all building/pushing image targets
VERSION_TAG ?= 3.8.0-rc.1
IMG ?= $(IMAGE_REGISTRY)/operator:$(VERSION_TAG)
BUNDLE_IMG ?= $(IMAGE_REGISTRY)/operator-bundle:$(VERSION_TAG)
BUNDLE_PACKAGE ?= iofog-operator
BUNDLE_CHANNEL ?= stable
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:crdVersions=v1,allowDangerousTypes=true"

# Local testing (no image build) — see hack/local/README.md
KUBECONFIG ?= $(HOME)/.kube/config
TEST_NAMESPACE ?= iofog-test
CR_PATH ?= config/cr/
LOCAL_CR_PATH ?= config/cr/local
POSTGRES_PATH ?= config/local/postgres
LOCAL_SCRIPTS ?= hack/local
KUBECTL ?= kubectl
export KUBECONFIG
export TEST_NAMESPACE

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
deploy-cr: create-namespace kustomize ## Deploy CR(s) from CR_PATH into TEST_NAMESPACE (uses KUBECONFIG)
	$(KUSTOMIZE) build $(CR_PATH) | $(KUBECTL) apply -f - -n $(TEST_NAMESPACE)

.PHONY: local-cluster-up
local-cluster-up: kustomize ## Create TEST_NAMESPACE and deploy Postgres (OrbStack/k3s local E2E)
	bash $(LOCAL_SCRIPTS)/cluster-up.sh

.PHONY: local-cluster-down
local-cluster-down: ## Delete TEST_NAMESPACE and all local E2E resources
	bash $(LOCAL_SCRIPTS)/cluster-down.sh

.PHONY: local-deploy-cr
local-deploy-cr: create-namespace kustomize ## Deploy local ControlPlane CR (config/cr/local)
	$(KUSTOMIZE) build $(LOCAL_CR_PATH) | $(KUBECTL) apply -f - -n $(TEST_NAMESPACE)

.PHONY: local-wait-baseline
local-wait-baseline: ## Wait until operator created Service, Ingress, Deployment, auth secret
	bash $(LOCAL_SCRIPTS)/wait-baseline.sh

.PHONY: local-scenario-a
local-scenario-a: ## E2E scenario A — service patch (controller + router)
	bash $(LOCAL_SCRIPTS)/scenario-service-patch.sh

.PHONY: local-scenario-b
local-scenario-b: ## E2E scenario B — ingress patch (host, class, TLS, annotations)
	bash $(LOCAL_SCRIPTS)/scenario-ingress-patch.sh

.PHONY: local-scenario-c
local-scenario-c: ## E2E scenario C — auth password patch + secret update
	bash $(LOCAL_SCRIPTS)/scenario-auth-patch.sh

.PHONY: local-scenarios
local-scenarios: ## Run E2E scenarios A, B, C in order (operator must be running)
	bash $(LOCAL_SCRIPTS)/run-scenarios.sh

.PHONY: local-e2e-setup
local-e2e-setup: local-cluster-up local-prep local-deploy-cr ## Cluster + CRDs + build + Postgres + ControlPlane CR
	@echo ""
	@echo "Setup complete. In another terminal run:  make run"
	@echo "Then wait for baseline:                 make local-wait-baseline"
	@echo "Run reconcile E2E tests:                  make local-scenarios"
	@echo "Full guide:                               hack/local/README.md"

.PHONY: run
run: build ## Run operator locally (uses KUBECONFIG, WATCH_NAMESPACE=TEST_NAMESPACE)
	WATCH_NAMESPACE=$(TEST_NAMESPACE) ./bin/iofog-operator

.PHONY: test-local
test-local: local-e2e-setup ## Alias for local-e2e-setup; then run operator and scenarios (see hack/local/README.md)

manifests: gen controller-gen rbac-manifests ## Generate manifests (optional FLAVOR=datasance|iofog; default both CRD sets)
ifdef FLAVOR
	$(MAKE) manifests-flavor FLAVOR=$(FLAVOR)
else
	$(MAKE) manifests-all
endif

.PHONY: manifests-all manifests-flavor gen-check rbac-manifests
manifests-all: ## Generate CRD manifests for both mirror flavors
	$(MAKE) manifests-flavor FLAVOR=datasance
	$(MAKE) manifests-flavor FLAVOR=iofog

manifests-flavor: controller-gen ## Generate CRD manifests for one flavor (FLAVOR=datasance|iofog)
	@test -n "$(FLAVOR)" || (echo "FLAVOR is required (datasance|iofog)" && exit 1)
	@case "$(FLAVOR)" in datasance|iofog) ;; *) echo "FLAVOR must be datasance or iofog" && exit 1 ;; esac
	@hack/gen-crds.sh $(FLAVOR)

rbac-manifests: controller-gen ## Generate RBAC and webhook manifests
	$(CONTROLLER_GEN) rbac:roleName=manager-role webhook paths="./..."

gen-check: gen manifests-all ## Verify committed CRDs match dual-flavor CRD output
	@git diff --exit-code -- config/crd/bases/ \
		|| (echo "ERROR: CRD drift — run 'make manifests' and commit" && exit 1)
	@echo "config/crd/bases/ is up to date"

fmt: ## Run gofmt against code
	@gofmt -s -w .

$(GOLANGCI_LINT):
	@echo "⬇️  Installing golangci-lint $(GOLANGCI_LINT_VERSION) → $(GOBIN)..."
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
		| sh -s -- -b $(GOBIN) $(GOLANGCI_LINT_VERSION)
	@echo "✓ golangci-lint $(GOLANGCI_LINT_VERSION) installed"

.PHONY: install-lint
install-lint: $(GOLANGCI_LINT) ## Install golangci-lint (pinned to GOLANGCI_LINT_VERSION)
	@$(GOLANGCI_LINT) version

lint: $(GOLANGCI_LINT) ## Run linters (auto-installs golangci-lint if needed)
	@echo "Running golangci-lint $(GOLANGCI_LINT_VERSION)..."
	@$(GOLANGCI_LINT) run --config .golangci.yaml ./...

.PHONY: security-code vulncheck quality
security-code: ## Static Go security analysis (gosec; not in golangci-lint)
	@echo "🔍 Running Go static security analysis..."
	@if ! command -v gosec >/dev/null 2>&1; then \
		echo "⬇️  Installing gosec $(GOSEC_VERSION)..."; \
		go install github.com/securego/gosec/v2/cmd/gosec@$(GOSEC_VERSION); \
	fi
	@gosec -exclude=G101 -exclude-generated $(GOSEC_SCOPE)

vulncheck: ## Dependency vulnerability scan (govulncheck + go mod verify)
	@echo "🔐 Running govulncheck..."
	@if ! command -v govulncheck >/dev/null 2>&1; then \
		echo "⬇️  Installing govulncheck $(GOVULNCHECK_VERSION)..."; \
		go install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION); \
	fi
	@govulncheck ./...
	@echo "🔍 Verifying module integrity..."
	@go mod verify

quality: lint security-code vulncheck ## Full quality gate (lint + gosec + govulncheck)

gen: controller-gen ## Generate code using controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

docker:
	docker build -t $(IMG) .

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

controller-gen: ## Install controller-gen
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.17.3 ;\
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

.PHONY: release-manifests release-manifests-all
release-manifests-all: kustomize ## Build manifest tarballs for both flavors (VERSION_TAG required)
	@test -n "$(VERSION_TAG)" || (echo "VERSION_TAG is required" && exit 1)
	@chmod +x hack/release-manifests.sh
	@IMAGE_REGISTRY=$(IMAGE_REGISTRY) hack/release-manifests.sh datasance $(VERSION_TAG)
	@IMAGE_REGISTRY=$(IMAGE_REGISTRY) hack/release-manifests.sh iofog $(VERSION_TAG)

release-manifests: kustomize ## Build manifest tarball for one flavor (FLAVOR + VERSION_TAG required)
	@test -n "$(FLAVOR)" || (echo "FLAVOR is required (datasance|iofog)" && exit 1)
	@test -n "$(VERSION_TAG)" || (echo "VERSION_TAG is required" && exit 1)
	@chmod +x hack/release-manifests.sh
	@IMAGE_REGISTRY=$(IMAGE_REGISTRY) hack/release-manifests.sh $(FLAVOR) $(VERSION_TAG)

.PHONY: bundle
bundle: kustomize ## Generate flavor-specific OLM bundle (FLAVOR=datasance|iofog required)
	@test -n "$(FLAVOR)" || (echo "FLAVOR is required (datasance|iofog)" && exit 1)
	@case "$(FLAVOR)" in datasance|iofog) ;; *) echo "FLAVOR must be datasance or iofog" && exit 1 ;; esac
	@test -n "$(VERSION_TAG)" || (echo "VERSION_TAG is required" && exit 1)
	@chmod +x hack/gen-bundle.sh
	@IMAGE_REGISTRY=$(IMAGE_REGISTRY) BUNDLE_PACKAGE=$(BUNDLE_PACKAGE) BUNDLE_CHANNEL=$(BUNDLE_CHANNEL) \
		hack/gen-bundle.sh $(FLAVOR) $(VERSION_TAG)


.PHONY: bundle-build
bundle-build: ## Build the bundle image.
	docker buildx build --platform=linux/amd64 -f bundle.Dockerfile -t $(BUNDLE_IMG) .

help: ## Show targets and flavor variables
	@echo "Flavor variables (override at build time):"
	@printf "  \033[33m%-35s\033[0m %s\n" OPERATOR_CRD_GROUP $(OPERATOR_CRD_GROUP)
	@printf "  \033[33m%-35s\033[0m %s\n" OPERATOR_COMPONENT_LABEL_DOMAIN $(OPERATOR_COMPONENT_LABEL_DOMAIN)
	@printf "  \033[33m%-35s\033[0m %s\n" IMAGE_REGISTRY $(IMAGE_REGISTRY)
	@printf "  \033[33m%-35s\033[0m %s\n" CONTROLLER_IMAGE_TAG $(CONTROLLER_IMAGE_TAG)
	@printf "  \033[33m%-35s\033[0m %s\n" ROUTER_IMAGE_TAG $(ROUTER_IMAGE_TAG)
	@printf "  \033[33m%-35s\033[0m %s\n" NATS_IMAGE_TAG $(NATS_IMAGE_TAG)
	@printf "  \033[33m%-35s\033[0m %s\n" VERSION_TAG $(VERSION_TAG)
	@echo ""
	@grep -h -E '^[a-zA-Z_.-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
