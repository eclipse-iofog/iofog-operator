# ioFog Operator Helm chart

Deploys the **ioFog operator** Deployment and optionally a **ControlPlane** custom resource. The operator reconciles ControlPlane specs into Controller, Router, NATS, Services, Secrets, and optional Ingress objects.

Published charts are **pre-stamped per mirror** at package time (`crdGroup`, `imageRegistry`, and component image tags). Install from your product mirror's gh-pages index. Do not override `crdGroup` or `imageRegistry` unless you run a custom build.

> **Migrating from [Datasance/helm](https://github.com/Datasance/helm)?** The legacy repo (`https://datasance.github.io/helm`, chart `pot`) is deprecated. Use the URLs below (`iofog-operator/iofog-operator`). v3.8 is greenfield — see [CHANGELOG.md](../../CHANGELOG.md#migration-greenfield-v380).

## Mirror URLs

| Mirror | `helm repo add` URL |
|--------|---------------------|
| Eclipse ioFog | `https://eclipse-iofog.github.io/iofog-operator` |
| Datasance PoT | `https://datasance.github.io/iofog-operator` |

Replace the URL in the examples below with your mirror.

## Prerequisites

- Kubernetes **1.22+**
- Helm **3.x**
- Pull access to the stamped container registry (operator, controller, router, nats)
- For NATS JetStream with persistent volumes: a default or configured `StorageClass`

## Download default values

The chart shipped for each release version has **stamped** defaults (registry, CRD group, image tags). Use one of:

```bash
# Print defaults for a version
helm show values iofog-operator/iofog-operator --version <VERSION>

# Save to a file for editing
helm show values iofog-operator/iofog-operator --version <VERSION> > my-values.yaml

# Download and unpack the full chart
helm pull iofog-operator/iofog-operator --version <VERSION> --untar
# → iofog-operator/values.yaml
```

From the gh-pages site (same content as the published chart for the latest release on that branch):

- `https://<mirror-host>/iofog-operator/values.yaml`
- `https://<mirror-host>/iofog-operator/values-<VERSION>.yaml`

Example (Datasance): `https://datasance.github.io/iofog-operator/values.yaml`

Machine-readable schema for editors and validation: [`values.schema.json`](values.schema.json) in this directory.

## Quick install

```bash
helm repo add iofog-operator https://eclipse-iofog.github.io/iofog-operator   # or datasance URL
helm repo update
helm install pot iofog-operator/iofog-operator \
  --namespace iofog-system --create-namespace \
  --version <VERSION> \
  --set controlplane.spec.auth.bootstrap.password='ReplaceMe1!'
```

Bootstrap password when `auth.mode=embedded`: at least **12 characters**, one **uppercase** letter, and one **special** character. Prefer a Secret — see [Auth](#auth-v38).

## Install with a values file

```bash
helm show values iofog-operator/iofog-operator --version <VERSION> > my-values.yaml
# edit my-values.yaml — set auth.bootstrap.password or passwordSecretRef, database, services, etc.
helm install pot iofog-operator/iofog-operator \
  --namespace iofog-system --create-namespace \
  --version <VERSION> \
  -f my-values.yaml
```

## Upgrade

```bash
helm upgrade pot iofog-operator/iofog-operator \
  --namespace iofog-system \
  --version <VERSION> \
  -f my-values.yaml
```

**v3.8 is greenfield** — there is no supported in-place upgrade from v3.7 operator or legacy `Datasance/helm` chart. Uninstall the old stack and CRDs before installing v3.8.

## LoadBalancer and externalTrafficPolicy

Chart defaults use **LoadBalancer** for `services.controller` and `services.router`, and **ClusterIP** for NATS client/server Services.

When `externalTrafficPolicy` is **omitted** in the ControlPlane spec, the operator applies:

| Service type | Default `externalTrafficPolicy` |
|--------------|----------------------------------|
| **LoadBalancer** | `Local` |
| **NodePort** | `Cluster` |
| **ClusterIP** | *(unset)* |

- **`Local`** — typical on cloud load balancers; preserves client source IP on nodes that receive traffic.
- **`Cluster`** — routes external traffic to any node; use when your cluster's LoadBalancer implementation does not work well with `Local` (for example some local or single-node Kubernetes distributions, lightweight LB controllers, or when Services stay `<pending>` with `Local`).

Set explicitly per service in values:

```yaml
controlplane:
  spec:
    services:
      controller:
        type: LoadBalancer
        externalTrafficPolicy: Cluster
      router:
        type: LoadBalancer
        externalTrafficPolicy: Cluster
      nats:
        type: LoadBalancer
        externalTrafficPolicy: Cluster
      natsServer:
        type: ClusterIP
```

Use **Ingress** instead of LoadBalancer when you have an ingress controller and prefer host-based routing — see [`controlplane.spec.ingresses`](#controlplanespecingresses-optional).

## Auth (v3.8)

Keycloak fields are removed. Use **embedded** local OIDC or an **external** IdP.

### Embedded (default)

```yaml
controlplane:
  spec:
    auth:
      mode: embedded
      insecureAllowHttp: false   # set true only for HTTP dev clusters
      bootstrap:
        username: admin
        password: ""             # required unless passwordSecretRef is set
```

Or reference a Secret:

```yaml
controlplane:
  spec:
    auth:
      mode: embedded
      bootstrap:
        username: admin
        passwordSecretRef:
          name: controller-bootstrap
          key: password
```

### External IdP

```yaml
controlplane:
  spec:
    auth:
      mode: external
      issuerUrl: https://auth.example.com/realms/myrealm
      client:
        id: controller
        secret: "..."
```

## PostgreSQL example

SQLite is the chart default. For production, use Postgres:

```yaml
controlplane:
  spec:
    database:
      provider: postgres
      user: admin
      host: postgres.postgres.svc.cluster.local
      port: 5432
      password: "..."              # or use a Secret via vault / external tooling
      databaseName: controller
      ssl: false
```

## Verify

```bash
kubectl get pods -n iofog-system
kubectl get svc -n iofog-system
kubectl get controlplanes.datasance.com -n iofog-system    # Datasance mirror
# kubectl get controlplanes.iofog.org -n iofog-system      # Eclipse mirror
```

Wait until the ControlPlane status shows **Ready** and component Deployments are available.

## Uninstall

```bash
helm uninstall pot -n iofog-system
kubectl delete controlplanes --all -n iofog-system   # if CRs remain
# Remove CRD only when no ControlPlane instances exist cluster-wide:
kubectl delete crd controlplanes.datasance.com        # or controlplanes.iofog.org
```

## Values reference

Top-level keys map to chart templates and the ControlPlane CR. Defaults in the packaged chart match your mirror and release version.

### Global

| Value | Description |
|-------|-------------|
| `nameOverride` | Short chart name override (default `iofog-operator`) |
| `fullnameOverride` | Full name for operator Deployment / ServiceAccount |
| `crdGroup` | ControlPlane API group (`datasance.com` or `iofog.org`) — stamped at package time |
| `imageRegistry` | Registry prefix for component images — stamped at package time |
| `imagePullSecrets` | Global pull secrets for all chart workloads |

### `crds.install`

Install the ControlPlane CRD with the chart (default `true`). Set `false` if the CRD is already installed cluster-wide (for example by a GitOps CRD bundle).

### `rbac.create`

Create operator Role, RoleBinding, and ServiceAccount (default `true`).

### `operator`

| Value | Description |
|-------|-------------|
| `operator.replicaCount` | Operator replicas (default `1`; leader election enabled) |
| `operator.image` | Operator container image — stamped at package time |
| `operator.imagePullPolicy` | `Always`, `IfNotPresent`, or `Never` |
| `operator.serviceAccount` | Create/name/annotations for the operator SA |
| `operator.resources` | CPU/memory requests and limits |
| `operator.nodeSelector` / `tolerations` / `affinity` | Pod placement |
| `operator.extraEnv` / `extraArgs` | Additional container env vars and args |
| `operator.podAnnotations` / `podLabels` | Pod metadata |
| `operator.podSecurityContext` / `securityContext` | Security contexts |

The operator watches the namespace it is deployed in (`WATCH_NAMESPACE` = pod namespace).

### `controlplane`

| Value | Description |
|-------|-------------|
| `controlplane.create` | Create a ControlPlane CR (default `true`) |
| `controlplane.name` | CR name (default `pot`; often matches release name) |
| `controlplane.namespace` | CR namespace (default: release namespace) |
| `controlplane.annotations` / `labels` | Metadata on the ControlPlane CR |

### `controlplane.spec.replicas`

| Value | Description |
|-------|-------------|
| `replicas.controller` | Controller Deployment replicas (default `1`) |
| `replicas.nats` | NATS StatefulSet replicas (default `2`, minimum `2` when NATS enabled) |

### `controlplane.spec.database`

Required in CRD v3. Default `provider: sqlite` needs no host.

| Field | Description |
|-------|-------------|
| `provider` | `sqlite` or `postgres` |
| `user`, `host`, `port`, `password`, `databaseName` | Postgres connection |
| `ssl`, `ca` | TLS to database |

### `controlplane.spec.auth`

See [Auth (v3.8)](#auth-v38). Optional embedded-only blocks: `rateLimit`, `sessionStore`, `tokenTtl`, `oidcTtl`, `consoleClient`, `consoleClientEnabled`.

### `controlplane.spec.events`

Audit and retention: `auditEnabled`, `retentionDays`, `cleanupInterval`, `captureIpAddress`.

### `controlplane.spec.images`

| Value | Description |
|-------|-------------|
| `images.controller` | Controller image (stamped at package time) |
| `images.router` | Router image |
| `images.nats` | NATS hub image |
| `images.pullSecret` | Pull secret for component pods |

### `controlplane.spec.services`

| Service key | Default type | Role |
|-------------|--------------|------|
| `services.controller` | LoadBalancer | Controller API and console |
| `services.router` | LoadBalancer | Router AMQP ports |
| `services.nats` | ClusterIP | NATS client / mesh ports |
| `services.natsServer` | ClusterIP | NATS server port |

Each supports `type`, `annotations`, and `externalTrafficPolicy` (`Local` or `Cluster` for LoadBalancer/NodePort). See [LoadBalancer and externalTrafficPolicy](#loadbalancer-and-externaltrafficpolicy).

### `controlplane.spec.nats`

| Value | Description |
|-------|-------------|
| `nats.enabled` | Deploy NATS hub StatefulSet (default `true`) |
| `nats.jetStream.memoryStoreSize` | JetStream memory store (default `1Gi` in operator if unset) |
| `nats.jetStream.storageSize` | Persistent volume size (default `10Gi` in operator if unset) |
| `nats.jetStream.storageClassName` | StorageClass for JetStream PVCs |

### `controlplane.spec.controller`

Runtime flags: `https`, `logLevel`, `publicUrl`, `trustProxy`, `consoleUrl`, `consolePort`, `pidBaseDir`, `secretName`. `publicUrl` can be inferred from Ingress or LoadBalancer when omitted.

### `controlplane.spec.ingresses` (optional)

Use when exposing via an ingress controller instead of (or in addition to) LoadBalancer.

**Controller ingress** — requires a non-empty `host` when TLS is used:

| Field | Description |
|-------|-------------|
| `ingresses.controller.host` | Ingress hostname |
| `ingresses.controller.ingressClassName` | Ingress class (e.g. `nginx`, `traefik`) |
| `ingresses.controller.secretName` | TLS secret name |
| `ingresses.controller.annotations` | Ingress annotations (e.g. cert-manager) |

**Router / NATS ingress** — address and port fields for hub registration when not using LoadBalancer.

If `services.controller.type` is `ClusterIP`, the operator creates a controller Ingress when ingress settings are present.

### `controlplane.spec.vault` (optional)

External secrets backends: `hashicorp`, `openbao`, `vault`, `aws`, `aws-secrets-manager`, `azure`, `azure-key-vault`, `google`, `google-secret-manager`. Configure `enabled`, `provider`, `basePath`, and the provider-specific block.

## Chart layout

```
charts/iofog-operator/
  Chart.yaml
  values.yaml              # default values (stamped in published chart)
  values.schema.json       # JSON Schema for values
  crds/                    # flavor-specific CRD (stamped at package time)
  templates/
    operator/              # Operator Deployment and ServiceAccount
    rbac/                  # Role and RoleBinding
    controlplane.yaml      # ControlPlane CR instance
```

## Related links

- Operator repository and release artifacts — see your mirror's GitHub org (`Datasance/iofog-operator` or `eclipse-iofog/iofog-operator`)
- Root [README.md](../../README.md) and [CONTRIBUTING](../../CONTRIBUTING) for dual-mirror CI variables
