# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [3.8.2] - 2026-07-23

### Changed

- **iofog-go-sdk** - bumped to **`github.com/eclipse-iofog/iofog-go-sdk/v3@v3.8.2`**.
- **Default component image tags** - operator and controller **3.8.2**; router **3.8.1**; NATS **2.14.3-1** (Makefile `LDFLAGS`, Helm `values.yaml`, OLM bundle CSV).
- **Release packaging** - Helm chart `version` / `appVersion`, `VERSION_TAG` defaults, and install docs updated to **3.8.2**.
- **Container base image** - refreshed UBI9 minimal digest in `Dockerfile`.

### Fixed

- **NATS default in samples and OLM bundle** - `config/cr/controlplane.yaml` and bundle CSV example now use **`2.14.3-1`** instead of stale **`2.12.4`** (Makefile and Helm were already correct in 3.8.1).

## [3.8.1] - 2026-07-10

### Changed

- **Go toolchain** - Go **1.26.5** (from 1.26.4) in `go.mod`, Dockerfile builder image, and CI workflows.
- **iofog-go-sdk** - bumped to **`github.com/eclipse-iofog/iofog-go-sdk/v3@v3.8.1`**.
- **Default component image tags** - operator, controller, and router **3.8.1**; NATS **2.14.3-1** (Makefile `LDFLAGS`, Helm `values.yaml`, OLM bundle CSV).
- **Release packaging** - Helm chart `version` / `appVersion`, `VERSION_TAG` defaults, and install docs updated to **3.8.1**.

### Fixed

- **NATS default image tag** - `NATS_IMAGE_TAG` and Helm defaults now use **`2.14.3-1`** instead of the incorrect **`3.8.0`** component-train tag.

## [3.8.0] - End of June 2026

### Added

- **Dual-mirror publish model** - identical source tree; Eclipse (`iofog.org/v3`) and Datasance (`datasance.com/v3`) flavors differ at CI publish time (container registry, CRD API group, Helm index URL, OCI labels).
- **ControlPlane auth schema** - `spec.auth.mode: embedded | external` with bootstrap credentials (`username`, `password`, optional `passwordSecretRef`) or external OIDC (`issuerUrl`, `client.id`, `client.secret`); optional embedded-only `consoleClient`, rate limits, and session store.
- **Controller v3.8 env mapping** - operator maps `AUTH_*`, `OIDC_*`, and `CONSOLE_*` env vars into the Controller pod per Controller `env-mapping.js`.
- **Operator → Controller API auth** - embedded: `POST /api/v3/user/login` with bootstrap creds from Secret; external: OAuth2 `client_credentials` via issuer `.well-known/openid-configuration` discovery.
- **NATS hub** - optional `spec.nats` block (JetStream storage, enable/disable); NATS services and ingress fields on ControlPlane.
- **Vault integration** - optional `spec.vault` block; operator creates credentials Secret and injects `VAULT_*` env vars.
- **In-repo Helm chart** at `charts/iofog-operator/`; published to each mirror's gh-pages on `v*` tags.
- **GitHub Actions CI** - `ci.yml` preflight on PR/`develop`; `release.yml` and `govulncheck.yml` on tags; `azure-pipelines.yml` removed.
- **OLM** - package `iofog-operator`, channel `stable`; flavor-aware bundle via `make bundle`.
- **Release artifacts per flavor** - manifest tarballs (`manifests-iofog-*`, `manifests-datasance-*`), OLM bundle images, Helm chart index entries.

### Deprecated

- **[Datasance/helm](https://github.com/Datasance/helm)** - standalone Helm repo (`https://datasance.github.io/helm`, chart `pot`) is retired. Charts ship from **`charts/iofog-operator/`** in this repository and publish to each mirror's gh-pages (`https://datasance.github.io/iofog-operator`, `https://eclipse-iofog.github.io/iofog-operator`). Redirect README stub: [`docs/helm/datasance-helm-deprecation-README.md`](docs/helm/datasance-helm-deprecation-README.md).

### Changed

- **Go toolchain** - Go **1.26.4**; module path **`github.com/eclipse-iofog/iofog-operator/v3`**; SDK **`github.com/eclipse-iofog/iofog-go-sdk/v3@v3.8.0`** train.
- **Default component images** - `ghcr.io/eclipse-iofog/*` (Eclipse) or `ghcr.io/datasance/*` (Datasance); `spec.images` overrides defaults.
- **Controller spec** - `publicUrl`, `trustProxy`, `consoleUrl`, `consolePort`; URL defaulting from ingress host or LoadBalancer IP.
- **Reconcile hot-path** - patch Services and Ingresses on spec change; restart Controller pods when auth or DB secrets change.
- **Copyright** - per-file headers removed; sole attribution in root `NOTICE`.
- **NATS TLS env** - `NATS_SSL_DIR` renamed to `NATS_TLS_DIR` (align with Controller v3.8).

### Removed

- **v3.7 in-place upgrade path** - see [Migration](#migration-greenfield-v380) below.
- **Application CRD and reconciler** - operator scope is **ControlPlane only**; deploy applications via **potctl**, **iofogctl**, or **iofog-go-sdk** (`pkg/apps`). `OPERATOR_DEPLOY_API_VERSION` build arg removed.
- **Keycloak auth fields** on ControlPlane - `auth.url`, `realm`, `realmKey`, `ssl`, `viewerClient`; all `KC_*` Controller env vars.
- **Keycloak Admin API** - `UpdateECNViewerClientRootURL` and console client IdP registration removed; `NoopUpdater` stub for future OIDC admin.
- **Controller ecnViewer fields** - `ecnViewerPort`, `ecnViewerUrl` dropped from ControlPlane spec.
- **Legacy PoT runtime identifiers** - `pot` naming scrubbed from labels, leader election ID, and samples (product strings `Datasance PoT` / `Eclipse ioFog` remain in docs/CSV where appropriate).

### Security

- Bootstrap passwords stored in Kubernetes Secrets; never logged or written to CR status.
- `make quality` gate: golangci-lint, gosec, govulncheck.

### Migration (greenfield v3.8.0)

**v3.8.0 is a greenfield release.** There is no supported upgrade from v3.7.x.

1. **Uninstall** the legacy operator deployment and **delete v3.7 CRDs** (`ControlPlane`, `Application`, and any flavor-specific variants).
2. **Rewrite ControlPlane manifests** - replace Keycloak `auth` block with embedded or external OIDC per [`config/cr/controlplane.yaml`](config/cr/controlplane.yaml) (flavor-specific samples ship in release manifest tarballs).
3. **Migrate application workloads** - export app definitions from removed `Application` CRs; redeploy with potctl, iofogctl, or the Go SDK. Pass API version per flavor (`iofog.org/v3` or `datasance.com/v3`) via CLI/SDK options.
4. **Install v3.8** using your mirror's channel:
   - **Helm (recommended)** - add the mirror-specific repo, then `helm install`:
     - Eclipse: `https://eclipse-iofog.github.io/iofog-operator`
     - Datasance: `https://datasance.github.io/iofog-operator`
   - **Manifest tarballs** - `manifests-iofog-<version>.tar.gz` or `manifests-datasance-<version>.tar.gz` from GitHub Releases.
   - **OLM** - `ghcr.io/eclipse-iofog/operator-bundle:<version>` or `ghcr.io/datasance/operator-bundle:<version>`.
5. **Deprecate [Datasance/helm](https://github.com/Datasance/helm)** - use the in-repo chart published to gh-pages above.

See [README.md](README.md) for install examples and dual-mirror workflow.

## [3.0.0] - 2022-05-09

### Changed

- Update CRDs and CR from v1beta1 to v1 to be Kubernetes v1.22+ compliant

## [3.0.0-beta1] - 2021-08-13

- No changes since alpha2

## [3.0.0-alpha2] - 2021-07-28

### Added

- Support for Base URLs for ioFog Controllers

## [3.0.0-alpha1] - 2021-03-11

### Added

- Reproduce entire project using Operator Framework v1.3.0
- Upgrade Operator SDK to 0.15.2 to avoid modules import error
- Add Status.Conditions to ControlPlane type and reconciler logic

### Changed

- Refactor ControlPlane reconciler runtime into more obvious state machine

### Removed

- Cluster Role Binding from Port Manager

## [2.0.1] - 2020-10-02

### Removed

- Router HTTP Port from Load Balancer

## [2.0.0] - 2020-08-05

### Added

- Remove Kubelet

### Fixed

- Increase LB timeouts
- Increase Controller readiness probe delay
- Change Router liveness probe to readiness
- Service accounts creation failure
- Fix rollout policies for all components

## [2.0.0-beta3] - 2020-04-23

### Added

- Use Proxy and Router images in Controller env vars
- Increase wait time for Router IP
- Refactor for parallel reconciliation

### Fixed

- Update go-sdk module with WaitForLoadBalancer fix
- Fix CR errors

## [2.0.0-beta2] - 2020-04-06

### Added

- Add retries to ioFog Controller client
- Add IsSupportedCustomResource
- Add Proxy service to ControlPlaneSpec
- Add CR helper functions to iofog pkg
- Refactor Kog to ControlPlane and make more optional fields in API type
- Add RouterImage to Kog spec

## [2.0.0-beta] - 2020-03-12

### Added

- Upgrade go-sdk to v2
- Make PVC creation optional
- Add PV for Controller sqlite db

## [2.0.0-alpha] - 2020-03-10

### Added

- Replace env var with API call to Controller for default Router
- Add port manager env vars
- Add Skupper loadbalancer IP and router ports to Controller env vars
- Add PortManagerImage to Kog ControlPlane
- Add env vars for Port Manager
- Add readiness probe to Port Manager
- Deploy Port Manager
- Deploy Skupper Router

### Fixed

- Consolidate usage of iofog client and reorganize controller reconciliation
- Removes all references to Connector

[Unreleased]: https://github.com/eclipse-iofog/iofog-operator/compare/v3.8.1...HEAD
[3.8.1]: https://github.com/eclipse-iofog/iofog-operator/compare/v3.8.0...v3.8.1
[3.8.0]: https://github.com/eclipse-iofog/iofog-operator/compare/v3.0.0...v3.8.0
[3.0.0]: https://github.com/eclipse-iofog/iofog-operator/compare/v3.0.0-beta1...v3.0.0
[3.0.0-beta1]: https://github.com/eclipse-iofog/iofog-operator/compare/v3.0.0-alpha2...v3.0.0-beta1
[3.0.0-alpha2]: https://github.com/eclipse-iofog/iofog-operator/compare/v3.0.0-alpha1...v3.0.0-alpha2
[3.0.0-alpha1]: https://github.com/eclipse-iofog/iofog-operator/compare/v2.0.1...v3.0.0-alpha1
[2.0.1]: https://github.com/eclipse-iofog/iofog-operator/compare/v2.0.0...v2.0.1
[2.0.0]: https://github.com/eclipse-iofog/iofog-operator/compare/v2.0.0-beta3...v2.0.0
[2.0.0-beta3]: https://github.com/eclipse-iofog/iofog-operator/compare/v2.0.0-beta2...v2.0.0-beta3
[2.0.0-beta2]: https://github.com/eclipse-iofog/iofog-operator/compare/v2.0.0-beta...v2.0.0-beta2
[2.0.0-beta]: https://github.com/eclipse-iofog/iofog-operator/compare/v2.0.0-alpha...v2.0.0-beta
[2.0.0-alpha]: https://github.com/eclipse-iofog/iofog-operator/tree/v2.0.0-alpha
