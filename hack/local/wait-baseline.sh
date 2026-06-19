#!/usr/bin/env bash
# Wait until operator has created baseline resources (after make run or local-deploy-operator + local-deploy-cr).

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=hack/local/lib.sh
source "$ROOT/hack/local/lib.sh"

require_kubectl
wait_baseline_resources

"$KUBECTL" get svc,ingress,deploy,secret -n "$TEST_NAMESPACE" \
  -l 'app.kubernetes.io/managed-by=iofog-operator' 2>/dev/null || true

log "Ready for scenarios: make local-scenario-a|b|c"
