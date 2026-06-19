# Local E2E — Plan 5-4 (reconcile hot-path)

Manual end-to-end tests for **service**, **ingress**, and **auth** patch behavior on a local Kubernetes cluster (OrbStack k3s). Uses the operator **binary** (`make run`) — no Docker image build required.

## Prerequisites

- OrbStack (or any k3s/k8s) with `kubectl` context working
- Go 1.26+ and repo tooling (`make` installs kustomize if missing)
- Pull access to `ghcr.io/datasance/*` component images (see `config/cr/local/controlplane.yaml`)

Verify cluster:

```bash
kubectl config current-context
kubectl get nodes
```

## Quick start (three terminals)

### Terminal 1 — one-time setup

```bash
make local-e2e-setup
```

This runs:

1. `make local-cluster-up` — namespace `iofog-test` + Postgres
2. `make local-prep` — install CRDs + build `bin/iofog-operator`
3. `make local-deploy-cr` — apply local ControlPlane CR

### Terminal 2 — operator

```bash
make run
```

Leave running. The operator watches `TEST_NAMESPACE` (default `iofog-test`).

### Terminal 1 (or 3) — wait, then test

```bash
make local-wait-baseline    # wait for Service, Ingress, Deployment, auth secret
make local-scenarios        # run scenarios A → B → C
```

## Step-by-step (replay from scratch)

| Step | Command | What it does |
|------|---------|--------------|
| 1 | `make local-cluster-up` | Creates `iofog-test`, deploys Postgres, waits until ready |
| 2 | `make local-prep` | Installs CRDs, builds operator binary |
| 3 | `make local-deploy-cr` | Applies `config/cr/local/controlplane.yaml` |
| 4 | `make run` *(other terminal)* | Starts operator against cluster |
| 5 | `make local-wait-baseline` | Waits for operator-created resources |
| 6 | `make local-scenario-a` | Service patch test |
| 7 | `make local-scenario-b` | Ingress patch test |
| 8 | `make local-scenario-c` | Auth secret patch test |
| 9 | `make local-cluster-down` | Deletes namespace when finished |

Or combine steps 6–8: `make local-scenarios`.

## Test scenarios

### Scenario A — Service patch

**Script:** `hack/local/scenario-service-patch.sh` · **Make:** `make local-scenario-a`

**Change:** Patch `spec.services.controller` annotations + `externalTrafficPolicy`, then `spec.services.router` annotations.

**Pass criteria:**

- Service UIDs unchanged (no delete/recreate)
- `controller` Service has `test.io/patch: phase-5-4` and `externalTrafficPolicy: Local`
- `router` Service has `test.io/router-patch: phase-5-4`
- ControlPlane `metadata.generation` increases

**Replay alone** (after baseline is ready):

```bash
make local-scenario-a
```

To re-test from a clean ControlPlane spec, edit `config/cr/local/controlplane.yaml` and `kubectl apply`, or delete/recreate the namespace with `make local-cluster-down && make local-e2e-setup`.

### Scenario B — Ingress patch

**Script:** `hack/local/scenario-ingress-patch.sh` · **Make:** `make local-scenario-b`

**Change:** Patch `spec.ingresses.controller` host, `ingressClassName`, TLS `secretName`, annotations.

**Pass criteria:**

- Ingress `pot-controller` UID unchanged
- Host → `pot-v2.local`
- `ingressClassName` → `traefik`
- TLS secret → `pot-tls`
- Annotation `cert-manager.io/cluster-issuer` → `local-test`

**Note:** OrbStack/k3s ships **without an ingress controller** by default. The local CR uses **controller `LoadBalancer`** so deploy can reach Ready without ingress. Scenario B temporarily switches to `ClusterIP` to create `pot-controller`; the operator may log `no LoadBalancer ingress found` — that is expected and does not block **spec** patch verification.

### Scenario C — Auth patch

**Script:** `hack/local/scenario-auth-patch.sh` · **Make:** `make local-scenario-c`

**Change:** Patch `spec.auth.bootstrap.password` from `LocalTest12!` → `LocalTest13!` (≥12 characters required by Controller).

**Pass criteria:**

- Secret `controller-auth-credentials` key `auth-bootstrap-password` updated
- ControlPlane generation increases

## Configuration

| Variable | Default | Purpose |
|----------|---------|---------|
| `TEST_NAMESPACE` | `iofog-test` | Namespace for Postgres + ControlPlane |
| `KUBECONFIG` | `~/.kube/config` | Cluster credentials |
| `LOCAL_CR_PATH` | `config/cr/local` | Kustomize overlay for ControlPlane CR |
| `POSTGRES_PATH` | `config/local/postgres` | Postgres manifests |
| `RECONCILE_WAIT_SECS` | `15` | Wait after patch before assertion |

**OrbStack/k3s:**
- Set `spec.services.router.externalTrafficPolicy: Cluster` (and controller too when using `LoadBalancer`). `Local` can leave the Service stuck at `<pending>`.

Example:

```bash
make local-cluster-up TEST_NAMESPACE=my-test
make run TEST_NAMESPACE=my-test
```

## Files

```
config/cr/local/controlplane.yaml   # Local ControlPlane CR (LoadBalancer controller for OrbStack)
config/local/postgres/              # Postgres for controller DB
hack/local/
  cluster-up.sh                     # Namespace + Postgres
  cluster-down.sh                   # Delete namespace
  wait-baseline.sh                  # Wait for operator resources
  scenario-service-patch.sh         # Scenario A
  scenario-ingress-patch.sh         # Scenario B
  scenario-auth-patch.sh            # Scenario C
  run-scenarios.sh                  # A + B + C
  lib.sh                            # Shared helpers
```

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| `Timed out waiting for baseline resources` | Ensure `make run` is running in another terminal |
| Postgres not ready | `kubectl get pods -n iofog-test` — re-run `make local-cluster-up` |
| Image pull errors | Log in to GHCR or edit images in `config/cr/local/controlplane.yaml` |
| Scenario fails after prior run | Scenarios are cumulative; run `make local-cluster-down` and setup again for a clean run |
| Operator not reconciling | Check `WATCH_NAMESPACE` matches `TEST_NAMESPACE` |

## Teardown

```bash
make local-cluster-down   # removes iofog-test namespace
make uninstall            # optional: remove CRDs
```

## Optional — in-cluster operator (Docker)

After binary tests pass:

```bash
make docker
make deploy
```

Stop the local `make run` process first. Re-run scenarios against the in-cluster operator.
