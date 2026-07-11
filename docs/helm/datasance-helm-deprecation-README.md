# Datasance PoT Helm chart - deprecated

> **Copy this file to [Datasance/helm](https://github.com/Datasance/helm) `README.md` in a separate PR** (final v3.7.2 redirect). Do not publish from this repo.

The standalone [Datasance/helm](https://github.com/Datasance/helm) repository is **deprecated** as of ioFog Operator **v3.8.1**. Charts are maintained in-repo and published from each product mirror on release tags.

## Use the new Helm repository

| Mirror | `helm repo add` URL | Chart |
|--------|---------------------|-------|
| Datasance PoT | `https://datasance.github.io/iofog-operator` | `iofog-operator/iofog-operator` |
| Eclipse ioFog | `https://eclipse-iofog.github.io/iofog-operator` | `iofog-operator/iofog-operator` |

```bash
helm repo add iofog-operator https://datasance.github.io/iofog-operator
helm repo update
helm install pot iofog-operator/iofog-operator \
  --namespace iofog-system --create-namespace \
  --version 3.8.1 \
  --set controlplane.spec.auth.bootstrap.password='ReplaceMe1!'
```

Documentation: [charts/iofog-operator/README.md](https://github.com/Datasance/iofog-operator/blob/develop/charts/iofog-operator/README.md) in **Datasance/iofog-operator** (or the Eclipse mirror).

## Migration from v3.7 (`datasance/pot`)

**v3.8.1 is greenfield** - there is no supported in-place upgrade from the legacy `pot` chart or Keycloak-shaped `auth` values.

1. Uninstall the v3.7 release and remove legacy CRDs if no longer needed.
2. Rewrite ControlPlane auth for embedded or external OIDC (see operator README / CHANGELOG).
3. Install from the new repository above.

## Legacy v3.7.2 reference (archived)

The last chart published from this repo targeted [iofog-operator 3.7.2](https://github.com/Datasance/iofog-operator/releases/tag/v3.7.2) with chart name **`pot`** and repo URL **`https://datasance.github.io/helm`**. That index is frozen; do not use it for new clusters.
