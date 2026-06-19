# ioFog Operator Helm chart

Deploys the ioFog operator and optionally a `ControlPlane` custom resource instance.

Published charts are **pre-stamped per mirror** at package time (CRD API group, container registry, and component images). Install from your product mirror's gh-pages URL — do not override `crdGroup` or `imageRegistry` unless you know you need a custom build.

> **Migrating from [Datasance/helm](https://github.com/Datasance/helm)?** The old `datasance/pot` chart (`https://datasance.github.io/helm`) is deprecated. Use the mirror URLs below (`iofog-operator/iofog-operator`). v3.8 is greenfield — see [CHANGELOG.md](../../CHANGELOG.md#migration-greenfield-v380).

| Mirror | `helm repo add` URL |
|--------|---------------------|
| Eclipse ioFog | `https://eclipse-iofog.github.io/iofog-operator` |
| Datasance PoT | `https://datasance.github.io/iofog-operator` |

## Prerequisites

- Kubernetes **1.22+**
- Helm **3.x**

## Install

```bash
helm repo add iofog-operator https://eclipse-iofog.github.io/iofog-operator   # or datasance URL
helm repo update
helm install pot iofog-operator/iofog-operator \
  --namespace iofog-system --create-namespace \
  --version 3.8.0 \
  --set controlplane.spec.auth.bootstrap.password='ReplaceMe1!'
```

## Auth (v3.8)

Keycloak fields are removed. Use embedded local OIDC or an external IdP.

**Embedded (default)** — set bootstrap credentials:

```yaml
controlplane:
  spec:
    auth:
      mode: embedded
      bootstrap:
        username: admin
        password: ""   # required: ≥12 chars, 1 upper, 1 special — or passwordSecretRef
```

**External** — point at your issuer and client credentials:

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

## Common values

| Value | Description |
|-------|-------------|
| `controlplane.create` | Create a ControlPlane CR (default `true`) |
| `controlplane.name` | ControlPlane resource name (defaults to release name) |
| `controlplane.spec.replicas.controller` | Controller replicas |
| `controlplane.spec.replicas.nats` | NATS replicas (min 2 when NATS enabled) |
| `controlplane.spec.services.*.type` | Service types (`LoadBalancer`, `ClusterIP`, etc.) |
| `controlplane.spec.database` | Database provider settings (sqlite by default) |
| `crds.install` | Install ControlPlane CRD with the chart (default `true`) |

See `values.yaml` and `values.schema.json` for the full surface.

## Verify

```bash
kubectl get pods -n iofog-system
kubectl get controlplanes.<your-crd-group> -n iofog-system
```

Use `controlplanes.iofog.org` on the Eclipse mirror or `controlplanes.datasance.com` on Datasance.

## Uninstall

**v3.8 is greenfield** — no upgrade path from v3.7. To remove:

```bash
helm uninstall pot -n iofog-system
kubectl delete controlplanes --all -n iofog-system   # if CRs remain
# Remove CRD only when no ControlPlane instances exist cluster-wide
kubectl delete crd controlplanes.iofog.org              # or controlplanes.datasance.com
```

## Chart layout

```
templates/
  operator/          Operator Deployment and ServiceAccount
  rbac/              Role and RoleBinding
  controlplane.yaml  ControlPlane CR instance
crds/                Flavor-specific CRD (stamped at package time)
```
