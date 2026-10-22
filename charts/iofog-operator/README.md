# ioFog Operator Helm chart

Deploys the **ioFog operator** and a **ControlPlane**. The operator reconciles that ControlPlane into Controller, Router, NATS, Services, Secrets, and an Ingress when the controller Service is ClusterIP.

Published charts are stamped per mirror at package time: CRD group, default ControlPlane name, and image tags. Install from your mirror. `values.yaml` is the Helm contract. [`marketplace-values.yaml`](marketplace-values.yaml) is the marketplace form (`x-form-schemas`) and is not a second `-f` file.

| Mirror | Helm repo | CRD group | ControlPlane name when `name` is empty |
|--------|-----------|-----------|----------------------------------------|
| Datasance PoT | `https://datasance.github.io/iofog-operator` | `datasance.com` | `pot` |
| Eclipse ioFog | `https://eclipse-iofog.github.io/iofog-operator` | `iofog.org` | `iofog` |

> **Migrating from [Datasance/helm](https://github.com/Datasance/helm)?** That chart is deprecated. v3.8 is greenfield — see [CHANGELOG.md](../../CHANGELOG.md).

## Prerequisites

- Kubernetes **1.22+**
- Helm **3.x**
- Pull access to the stamped registry (operator, controller, router, nats)
- A default StorageClass for the NATS JetStream volume, or set `nats.storageClassName`

## Install

```bash
helm repo add iofog-operator https://datasance.github.io/iofog-operator   # or the Eclipse URL
helm repo update
helm install pot iofog-operator/iofog-operator \
  --namespace iofog-system --create-namespace \
  --version <VERSION> \
  --set auth.bootstrap.password='ReplaceMe1!x'
```

The bootstrap password needs at least **12 characters**, one **uppercase** letter, and one **special** character. To keep it out of the Helm release, create a Secret and set `auth.bootstrap.passwordSecretRef` instead of `password`.

`name` overrides the ControlPlane resource name. Leave it empty to use `pot` or `iofog` for the mirror you installed. The Helm release name does not set that name.

```bash
helm show values iofog-operator/iofog-operator --version <VERSION> > my-values.yaml
helm upgrade pot iofog-operator/iofog-operator \
  --namespace iofog-system \
  --version <VERSION> \
  -f my-values.yaml
```

Marketplace form for the same defaults: `marketplace-values.yaml` on the chart repository site (`marketplace-values-<VERSION>.yaml` for one version).

**v3.8 is greenfield.** Uninstall a v3.7 operator and its CRDs before installing this chart.

## Values

`values.yaml` is a flat form. The templates build the ControlPlane spec from it. PEM text, gibibyte integers, and the cleanup interval in hours are not copied onto the CR as typed. Annotation maps and `key: value` lines become a Kubernetes annotation map. Advanced checkboxes exist only on the marketplace form.

### `name`

ControlPlane resource name. Empty uses the flavor default above.

### `images`

| Value | Role |
|-------|------|
| `images.operator` | Operator Deployment. Not a ControlPlane field. |
| `images.controller` | Controller |
| `images.router` | Router |
| `images.nats` | NATS |
| `images.pullSecret` | One Secret name, used by the operator pod and by Controller, Router, and NATS |

### `auth`

`mode` is `embedded` or `external`. `insecureAllowHttp` allows an HTTP OIDC issuer for either mode.

Embedded requires `bootstrap.username` and either `bootstrap.password` or:

```yaml
auth:
  mode: embedded
  bootstrap:
    username: admin
    passwordSecretRef:
      name: controller-bootstrap
      key: password
```

`passwordSecretRef` is not on the marketplace form. Rate limit, token TTL, OIDC TTL, and session store are embedded-only. A zero omits that field and keeps the controller default.

External requires `issuerUrl`, `client.id`, and `client.secret`. `issuerCa` is a base64-encoded PEM CA for a private issuer. Leave it empty for a public CA.

### `database`

`sqlite` needs no connection fields. The chart still writes the empty host, user, password, database name, and port `0` the CRD carries, and controller replicas must stay **1**.

`postgres` and `mysql` require host, database name, user, and password. Port `0` becomes **5432** or **3306**. `ca` is a base64-encoded PEM and is written only when `ssl` is true.

### `replicas`

`controller` defaults to 1. `nats` minimum is 2.

### `services`

| Service | Default | Exposure |
|---------|---------|----------|
| `services.controller` | LoadBalancer | API and console. ClusterIP creates a Kubernetes Ingress and requires `controller.https` plus a certificate. The Secret name is written to `ingresses.controller.secretName`. NodePort requires `controller.publicUrl`. |
| `services.router` | LoadBalancer | ClusterIP and NodePort require `services.router.ingress.address`. That address is registered as the default router. It is not an Ingress object. |
| `services.nats` | LoadBalancer | TLS ports (cluster, leaf, MQTT). ClusterIP and NodePort require `services.nats.ingress.address` for hub registration. Not an Ingress object. |
| `services.natsServer` | ClusterIP | In-cluster client `4222` and monitor `8222`, without TLS. An empty type is written as ClusterIP. LoadBalancer and NodePort accept `externalTrafficPolicy`. |

Leave `annotations` null. Set a map, or a string with one `key: value` per line. The marketplace textarea submits that string. Helm cannot replace a default map with a string, so the empty value stays null. Blank lines are skipped. A duplicate key fails the render. In a map, quote values that YAML would read as booleans or numbers, such as `"on"` or `"443"`. `externalTrafficPolicy` is `auto`, `Local`, or `Cluster`. `auto` omits the field. The operator then uses Local for LoadBalancer and Cluster for NodePort, and sets nothing for ClusterIP.

Router ingress ports default to message `5671`, interior `55671`, edge `45671`. NATS ingress ports default to server `4222`, cluster `6222`, leaf `7422`, MQTT `8883`, HTTP `8222`.

### `controller`

| Value | Role |
|-------|------|
| `logLevel` | `debug`, `info`, `warn`, or `error`. Default `info`. |
| `trustProxy` | `auto`, `on`, or `off`. `auto` omits the field so an ingress can turn trust proxy on. |
| `https` | Default `false`. ClusterIP requires `true`. |
| `publicUrl` | Required for NodePort, and for LoadBalancer when HTTPS is on. Otherwise the operator fills it from the ingress host or the load balancer address. |
| `consoleUrl` | Optional. Empty follows the public URL. |
| `tls.source` | `existing` or `create`, required when HTTPS is on. |

`existing` writes `tls.secretName` on the controller and, when the controller Service is ClusterIP, on the Ingress. For cert-manager, that name is the Secret the issuer will create, such as `pot-tls`. `create` takes PEM `tls.crt`, `tls.key`, and optional PEM `tls.ca`. The chart base64-encodes them into Secret `controller-tls` and sets that name on the controller and on the controller ingress.

### `nats`

`memoryStoreSize` and `storageSize` are whole numbers of Gi (`1` becomes `1Gi`). Defaults are 1 and 10. `storageClassName` empty uses the cluster default StorageClass.

### `vault`

When `enabled` is false the chart omits `spec.vault`. Otherwise `provider` is `hashicorp`, `openbao`, `aws`, `azure`, or `google`. OpenBao uses the `hashicorp` address, mount, and token fields and is sent as `provider: openbao`.

### `events`

Defaults: audit on, retention 14 days, cleanup every 24 hours, capture IP on. `cleanupIntervalHours` minimum is 2. The chart writes `spec.events.cleanupInterval` in seconds.

## Verify

```bash
kubectl get pods -n iofog-system
kubectl get controlplanes.datasance.com -n iofog-system    # Datasance
# kubectl get controlplanes.iofog.org -n iofog-system     # Eclipse
```

## Uninstall

```bash
helm uninstall pot -n iofog-system
kubectl delete controlplanes --all -n iofog-system
# Remove the CRD only when no ControlPlane remains in the cluster:
kubectl delete crd controlplanes.datasance.com    # or controlplanes.iofog.org
```

Helm does not upgrade CRDs on `helm upgrade`. The CRD in this chart is installed with the release.

## Chart layout

```
charts/iofog-operator/
  Chart.yaml
  values.yaml                 # Helm values
  marketplace-values.yaml     # marketplace form, not used by helm install
  templates/_flavor.tpl       # CRD group, ControlPlane name, operator image fallback
  templates/controlplane.yaml # builds the ControlPlane spec
  crds/                       # flavor CRD, replaced at package time
```
