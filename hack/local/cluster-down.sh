#!/usr/bin/env bash
# Tear down local E2E namespace (ControlPlane, postgres, and all child resources).

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=hack/local/lib.sh
source "$ROOT/hack/local/lib.sh"

require_kubectl

print_header "Tear down local E2E namespace (${TEST_NAMESPACE})"

if "$KUBECTL" get namespace "$TEST_NAMESPACE" >/dev/null 2>&1; then
  log "Deleting namespace ${TEST_NAMESPACE} (this removes ControlPlane and postgres)..."
  "$KUBECTL" delete namespace "$TEST_NAMESPACE" --wait=true
  log "Namespace deleted."
else
  log "Namespace ${TEST_NAMESPACE} does not exist — nothing to do."
fi

log "CRDs remain installed. Remove with: make uninstall"
