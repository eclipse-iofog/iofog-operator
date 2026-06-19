# ioFog Operator

[![CI](https://github.com/eclipse-iofog/iofog-operator/actions/workflows/ci.yml/badge.svg?branch=develop)](https://github.com/eclipse-iofog/iofog-operator/actions/workflows/ci.yml)
[![Release](https://github.com/eclipse-iofog/iofog-operator/actions/workflows/release.yml/badge.svg)](https://github.com/eclipse-iofog/iofog-operator/actions/workflows/release.yml)
[![Go](https://img.shields.io/badge/Go-1.26.4-blue.svg)](https://go.dev/)
[![License](https://img.shields.io/badge/License-EPL--2.0-blue.svg)](LICENSE)
[![govulncheck](https://github.com/eclipse-iofog/iofog-operator/actions/workflows/govulncheck.yml/badge.svg)](https://github.com/eclipse-iofog/iofog-operator/actions/workflows/govulncheck.yml)

[![Container (Eclipse)](https://img.shields.io/badge/ghcr.io-eclipse--iofog%2Foperator-2496ED?style=flat&logo=docker&logoColor=white)](https://ghcr.io/eclipse-iofog/operator)
[![Container (Datasance)](https://img.shields.io/badge/ghcr.io-datasance%2Foperator-2496ED?style=flat&logo=docker&logoColor=white)](https://ghcr.io/datasance/operator)
[![Helm (Eclipse)](https://img.shields.io/badge/Helm-eclipse--iofog%2Fiofog--operator-0f1689?style=flat&logo=helm&logoColor=white)](https://eclipse-iofog.github.io/iofog-operator)
[![Helm (Datasance)](https://img.shields.io/badge/Helm-datasance%2Fiofog--operator-0f1689?style=flat&logo=helm&logoColor=white)](https://datasance.github.io/iofog-operator)

**Upstream:** [eclipse-iofog/iofog-operator](https://github.com/eclipse-iofog/iofog-operator) · **Datasance mirror:** [Datasance/iofog-operator](https://github.com/Datasance/iofog-operator)

Kubernetes operator for **Eclipse ioFog** and **Datasance PoT** control planes. It reconciles `ControlPlane` custom resources to deploy Controller, Router, NATS, and related cluster resources. Application workloads are deployed via potctl, iofogctl, or the ioFog Go SDK - not through this operator.

## Container images

Identical git tree; registry, CRD API group, and Helm index URL differ by mirror CI variables only.

| Mirror | Operator image | OLM bundle image |
|--------|----------------|------------------|
| Eclipse ioFog (upstream) | `ghcr.io/eclipse-iofog/operator` | `ghcr.io/eclipse-iofog/operator-bundle` |
| Datasance (PoT-facing) | `ghcr.io/datasance/operator` | `ghcr.io/datasance/operator-bundle` |

Release tags use the semver without a `v` prefix (e.g. `:3.8.0`). Images, manifest tarballs, OLM bundles, and Helm charts publish on **`v*` tags** from each mirror's `release.yml`.

## Dual-mirror workflow

1. Develop on **`Datasance/iofog-operator`** branch **`develop`** (or feature branches `operator/<plan>-<topic>`).
2. Open PR to **`eclipse-iofog/iofog-operator`** **`develop`**.
3. CI runs on `develop` and PRs - **no GHCR push** on branch builds.
4. After merge, tag identical **`v*`** releases on both remotes; **`release.yml`** publishes images, manifest tarballs, OLM bundle, and Helm chart.

See [CONTRIBUTING](CONTRIBUTING) for CI repository variables and contributor workflow.

## Prerequisites

| Tool | Version |
|------|---------|
| Kubernetes | **1.22+** |
| Go | **1.26.4** (see `go.mod`) |
| Helm | **3.x** (for Helm install) |

## Greenfield release (v3.8)

**v3.8.0 is a greenfield release.** There is no in-place upgrade path from v3.7. Uninstall the legacy operator and CRDs before installing v3.8. The v3.8 operator manages **ControlPlane** resources only; the Application CRD and legacy auth fields are removed.

## Installation

Choose the channel that matches your fleet - Eclipse (`iofog.org/v3`) or Datasance (`datasance.com/v3`). Artifacts are published to GitHub Releases and mirror-specific registries on `v*` tags.

### Helm (recommended)

Helm charts are published to each mirror's gh-pages index. The legacy standalone repo [Datasance/helm](https://github.com/Datasance/helm) (`datasance.github.io/helm`, chart `pot`) is **deprecated** - use the URLs below instead.

| Mirror | `helm repo add` URL |
|--------|---------------------|
| Eclipse ioFog | `https://eclipse-iofog.github.io/iofog-operator` |
| Datasance | `https://datasance.github.io/iofog-operator` |

```bash
# Eclipse example - use datasance.github.io URL for the Datasance mirror
helm repo add iofog-operator https://eclipse-iofog.github.io/iofog-operator
helm repo update
helm install iofog-operator iofog-operator/iofog-operator \
  --namespace iofog-system --create-namespace \
  --version 3.8.0
```

### Manifest tarballs

GitHub Releases attach flavor-specific tarballs: `manifests-iofog-<version>.tar.gz` and `manifests-datasance-<version>.tar.gz`.

```bash
VERSION=3.8.0
FLAVOR=iofog   # or datasance

curl -fsSL -o manifests.tar.gz \
  "https://github.com/eclipse-iofog/iofog-operator/releases/download/v${VERSION}/manifests-${FLAVOR}-${VERSION}.tar.gz"
tar xzf manifests.tar.gz
kubectl apply -f "manifests-${FLAVOR}-${VERSION}/crds/"
kubectl apply -f "manifests-${FLAVOR}-${VERSION}/operator/install.yaml"
```

Adapt and apply `manifests-*/samples/controlplane.yaml` for your cluster.

### OLM

OLM bundles are published as container images (`operator-bundle:<version>`). Package **`iofog-operator`**, channel **`stable`**.

```bash
# Cluster must have OLM installed (Operator Lifecycle Manager)
VERSION=3.8.0
REGISTRY=ghcr.io/eclipse-iofog   # or ghcr.io/datasance

operator-sdk run bundle "${REGISTRY}/operator-bundle:${VERSION}" \
  --namespace iofog-system --timeout 5m
```

For catalog-based installs, index the bundle image from your mirror registry into an `OperatorGroup`-scoped catalog source.

## Local development

```bash
make quality    # lint + gosec + govulncheck
make build
make test
make run        # off-cluster; uses KUBECONFIG and WATCH_NAMESPACE
```

**Local E2E** (reconcile hot-path): see [hack/local/README.md](hack/local/README.md) for the OrbStack/k3s checklist (`make local-e2e-setup`, `make run`, `make local-scenarios`).

Run `make help` for all targets (manifest generation, bundle, release tarballs).

## License

Eclipse Public License 2.0 - see [LICENSE](LICENSE) and [NOTICE](NOTICE).
