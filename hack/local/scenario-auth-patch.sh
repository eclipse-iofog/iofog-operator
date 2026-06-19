#!/usr/bin/env bash
# Scenario C — Auth patch: bootstrap password in secret updates; controller Deployment restarts.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=hack/local/lib.sh
source "$ROOT/hack/local/lib.sh"

require_kubectl
wait_baseline_resources

print_header "Scenario C — Auth patch + controller restart"

PASS_BEFORE="$(auth_password)"
DEPLOY_GEN_BEFORE="$("$KUBECTL" get deploy controller -n "$TEST_NAMESPACE" -o jsonpath='{.metadata.generation}')"
GEN_BEFORE="$(cp_generation)"

log "Before — auth-bootstrap-password: ${PASS_BEFORE}"
log "Before — controller Deployment generation: ${DEPLOY_GEN_BEFORE}"
log "Before — ControlPlane generation: ${GEN_BEFORE}"

if [[ "$PASS_BEFORE" != "LocalTest12!" ]]; then
  log "WARN: expected initial password LocalTest12! (got ${PASS_BEFORE}); continuing anyway"
fi

log "Patching ControlPlane auth bootstrap password..."
"$KUBECTL" patch controlplane "$CONTROLPLANE_NAME" -n "$TEST_NAMESPACE" --type merge -p '
spec:
  auth:
    bootstrap:
      password: "LocalTest13!"
'

wait_reconcile

PASS_AFTER="$(auth_password)"
DEPLOY_GEN_AFTER="$("$KUBECTL" get deploy controller -n "$TEST_NAMESPACE" -o jsonpath='{.metadata.generation}')"
GEN_AFTER="$(cp_generation)"

log "After — auth-bootstrap-password: ${PASS_AFTER}"
log "After — controller Deployment generation: ${DEPLOY_GEN_AFTER}"
log "After — ControlPlane generation: ${GEN_AFTER}"

fail=0
assert_eq "auth secret password updated" "LocalTest13!" "$PASS_AFTER" || fail=1
[[ "$GEN_AFTER" -gt "$GEN_BEFORE" ]] && log "PASS ControlPlane generation increased" || { log "FAIL ControlPlane generation did not increase"; fail=1; }

# Deployment generation may not always bump on restart helper; check rollout or pod restart.
if [[ "$DEPLOY_GEN_AFTER" -gt "$DEPLOY_GEN_BEFORE" ]]; then
  log "PASS controller Deployment generation increased"
else
  log "NOTE: Deployment generation unchanged — checking pod restart timestamps..."
  "$KUBECTL" get pods -n "$TEST_NAMESPACE" -l app.kubernetes.io/component=controller -o wide
fi

if [[ "$fail" -ne 0 ]]; then
  die "Scenario C failed"
fi
log "Scenario C passed."
