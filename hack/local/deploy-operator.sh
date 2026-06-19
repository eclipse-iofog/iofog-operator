#!/usr/bin/env bash
# Deploy a published operator image into TEST_NAMESPACE for local E2E (in-cluster).
# Usage: IMG=ghcr.io/datasance/operator:3.8.0-beta.0 hack/local/deploy-operator.sh

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=hack/local/lib.sh
source "$ROOT/hack/local/lib.sh"

IMG="${IMG:-}"
KUSTOMIZE="${KUSTOMIZE:-kustomize}"
ROLLOUT_TIMEOUT_SECS="${ROLLOUT_TIMEOUT_SECS:-120}"

if [[ -z "$IMG" ]]; then
	die "IMG is required (e.g. IMG=ghcr.io/datasance/operator:3.8.0-beta.0)"
fi

command -v "$KUSTOMIZE" >/dev/null 2>&1 || die "kustomize not found (run make kustomize or set KUSTOMIZE=...)"

require_kubectl

if ! "$KUBECTL" get namespace "$TEST_NAMESPACE" >/dev/null 2>&1; then
	log "Creating namespace ${TEST_NAMESPACE}"
	"$KUBECTL" create namespace "$TEST_NAMESPACE"
fi

STAGE="$(mktemp -d)"
cleanup() {
	rm -rf "$STAGE"
}
trap cleanup EXIT

print_header "Deploy operator (${IMG}) into ${TEST_NAMESPACE}"

cp "$ROOT/config/operator/deployment.yaml" "$STAGE/deployment.yaml"
cp "$ROOT/config/operator/rbac.yaml" "$STAGE/rbac.yaml"
cat >"$STAGE/kustomization.yaml" <<'EOF'
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - deployment.yaml
  - rbac.yaml
EOF

(
	cd "$STAGE"
	"$KUSTOMIZE" edit set image "ghcr.io/datasance/operator=${IMG}"
	"$KUSTOMIZE" build . | "$KUBECTL" apply -n "$TEST_NAMESPACE" -f -
)

log "Waiting for iofog-operator Deployment rollout..."
"$KUBECTL" rollout status deploy/iofog-operator -n "$TEST_NAMESPACE" --timeout="${ROLLOUT_TIMEOUT_SECS}s"

log "Operator running in ${TEST_NAMESPACE} (${IMG})"
