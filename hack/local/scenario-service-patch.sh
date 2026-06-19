#!/usr/bin/env bash
# Scenario A - Service patch: annotations apply without Service delete.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=hack/local/lib.sh
source "$ROOT/hack/local/lib.sh"

require_kubectl
wait_baseline_resources

print_header "Scenario A - Service patch (controller annotations)"

UID_BEFORE="$("$KUBECTL" get svc controller -n "$TEST_NAMESPACE" -o jsonpath='{.metadata.uid}')"
GEN_BEFORE="$(cp_generation)"

log "Before - controller Service UID: ${UID_BEFORE}"
log "Before - ControlPlane generation: ${GEN_BEFORE}"

log "Patching ControlPlane (controller service annotations)..."
"$KUBECTL" patch controlplane "$CONTROLPLANE_NAME" -n "$TEST_NAMESPACE" --type merge -p '
spec:
  services:
    controller:
      type: ClusterIP
      annotations:
        test.io/patch: "phase-5-4"
'

fail=0
wait_for_patch \
  "[[ \"\$($KUBECTL get svc controller -n $TEST_NAMESPACE -o jsonpath='{.metadata.annotations.test\\.io/patch}')\" == \"phase-5-4\" ]]" \
  "controller annotation test.io/patch" 120 || fail=1

UID_AFTER="$("$KUBECTL" get svc controller -n "$TEST_NAMESPACE" -o jsonpath='{.metadata.uid}')"
ANNOTATION="$("$KUBECTL" get svc controller -n "$TEST_NAMESPACE" -o jsonpath='{.metadata.annotations.test\.io/patch}')"
GEN_AFTER="$(cp_generation)"

assert_eq "Service UID unchanged" "$UID_BEFORE" "$UID_AFTER" || fail=1
assert_eq "annotation test.io/patch" "phase-5-4" "$ANNOTATION" || fail=1
[[ "$GEN_AFTER" -gt "$GEN_BEFORE" ]] && log "PASS ControlPlane generation increased" || { log "FAIL ControlPlane generation did not increase"; fail=1; }

print_header "Scenario A - Service patch (router annotations + externalTrafficPolicy)"

ROUTER_UID_BEFORE="$("$KUBECTL" get svc router -n "$TEST_NAMESPACE" -o jsonpath='{.metadata.uid}')"
"$KUBECTL" patch controlplane "$CONTROLPLANE_NAME" -n "$TEST_NAMESPACE" --type merge -p '
spec:
  services:
    router:
      annotations:
        test.io/router-patch: "phase-5-4"
      externalTrafficPolicy: Local
'

wait_for_patch \
  "[[ \"\$($KUBECTL get svc router -n $TEST_NAMESPACE -o jsonpath='{.metadata.annotations.test\\.io/router-patch}')\" == \"phase-5-4\" ]]" \
  "router annotation test.io/router-patch" 120 || fail=1

ROUTER_ANNOTATION="$("$KUBECTL" get svc router -n "$TEST_NAMESPACE" -o jsonpath='{.metadata.annotations.test\.io/router-patch}')"
ROUTER_POLICY="$("$KUBECTL" get svc router -n "$TEST_NAMESPACE" -o jsonpath='{.spec.externalTrafficPolicy}')"
ROUTER_UID_AFTER="$("$KUBECTL" get svc router -n "$TEST_NAMESPACE" -o jsonpath='{.metadata.uid}')"

assert_eq "router Service UID unchanged" "$ROUTER_UID_BEFORE" "$ROUTER_UID_AFTER" || fail=1
assert_eq "router annotation" "phase-5-4" "$ROUTER_ANNOTATION" || fail=1
assert_eq "router externalTrafficPolicy" "Local" "$ROUTER_POLICY" || fail=1

if [[ "$fail" -ne 0 ]]; then
  die "Scenario A failed"
fi
log "Scenario A passed."
