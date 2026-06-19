#!/usr/bin/env bash
# Create test namespace and deploy Postgres for local E2E.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=hack/local/lib.sh
source "$ROOT/hack/local/lib.sh"

KUSTOMIZE="${KUSTOMIZE:-kustomize}"
POSTGRES_PATH="${POSTGRES_PATH:-$ROOT/config/local/postgres}"

require_kubectl

print_header "Step 1 - namespace + postgres (${TEST_NAMESPACE})"

if ! "$KUBECTL" get namespace "$TEST_NAMESPACE" >/dev/null 2>&1; then
  log "Creating namespace ${TEST_NAMESPACE}"
  "$KUBECTL" create namespace "$TEST_NAMESPACE"
else
  log "Namespace ${TEST_NAMESPACE} already exists"
fi

log "Applying Postgres manifests from ${POSTGRES_PATH}"
"$KUSTOMIZE" build "$POSTGRES_PATH" | "$KUBECTL" apply -n "$TEST_NAMESPACE" -f -

wait_postgres

log "Cluster prep complete."
log "Next: make local-prep && make local-deploy-cr"
log "Then in another terminal: make run"
