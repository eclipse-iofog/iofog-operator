#!/usr/bin/env bash
# Scenario B — Ingress patch: host, class, TLS secret, annotations apply without Ingress delete.
# OrbStack/k3s has no ingress controller by default; switch controller to ClusterIP to create
# the Ingress object (operator reconcile may log ingress LB status errors — patch spec is what we test).

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=hack/local/lib.sh
source "$ROOT/hack/local/lib.sh"

require_kubectl
wait_baseline_resources

ensure_controller_ingress() {
  if "$KUBECTL" get ingress pot-controller -n "$TEST_NAMESPACE" >/dev/null 2>&1; then
    return 0
  fi
  log "No pot-controller Ingress — switching controller Service to ClusterIP to create one..."
  "$KUBECTL" patch controlplane "$CONTROLPLANE_NAME" -n "$TEST_NAMESPACE" --type merge -p '
spec:
  services:
    controller:
      type: ClusterIP
  ingresses:
    controller:
      host: pot.local
      secretName: ""
      annotations: {}
'
  wait_for_patch \
    "$KUBECTL get ingress pot-controller -n $TEST_NAMESPACE >/dev/null 2>&1" \
    "pot-controller Ingress created" 120
}

ensure_controller_ingress

print_header "Scenario B — Ingress patch (pot-controller)"

UID_BEFORE="$("$KUBECTL" get ingress pot-controller -n "$TEST_NAMESPACE" -o jsonpath='{.metadata.uid}')"
HOST_BEFORE="$("$KUBECTL" get ingress pot-controller -n "$TEST_NAMESPACE" -o jsonpath='{.spec.rules[0].host}')"
GEN_BEFORE="$(cp_generation)"

log "Before — Ingress UID: ${UID_BEFORE}"
log "Before — host: ${HOST_BEFORE}"
log "Before — ControlPlane generation: ${GEN_BEFORE}"

log "Patching ControlPlane (ingress host, class, TLS secret, annotations)..."
"$KUBECTL" patch controlplane "$CONTROLPLANE_NAME" -n "$TEST_NAMESPACE" --type merge -p '
spec:
  ingresses:
    controller:
      ingressClassName: traefik
      host: pot-v2.local
      secretName: pot-tls
      annotations:
        cert-manager.io/cluster-issuer: local-test
'

fail=0
wait_for_patch \
  "[[ \"\$($KUBECTL get ingress pot-controller -n $TEST_NAMESPACE -o jsonpath='{.spec.rules[0].host}')\" == \"pot-v2.local\" ]]" \
  "ingress host pot-v2.local" 120 || fail=1

UID_AFTER="$("$KUBECTL" get ingress pot-controller -n "$TEST_NAMESPACE" -o jsonpath='{.metadata.uid}')"
HOST_AFTER="$("$KUBECTL" get ingress pot-controller -n "$TEST_NAMESPACE" -o jsonpath='{.spec.rules[0].host}')"
CLASS_AFTER="$("$KUBECTL" get ingress pot-controller -n "$TEST_NAMESPACE" -o jsonpath='{.spec.ingressClassName}')"
TLS_AFTER="$("$KUBECTL" get ingress pot-controller -n "$TEST_NAMESPACE" -o jsonpath='{.spec.tls[0].secretName}')"
ISSUER_AFTER="$("$KUBECTL" get ingress pot-controller -n "$TEST_NAMESPACE" -o jsonpath='{.metadata.annotations.cert-manager\.io/cluster-issuer}')"
GEN_AFTER="$(cp_generation)"

log "After — Ingress UID: ${UID_AFTER}"
log "After — host: ${HOST_AFTER}"
log "After — ingressClassName: ${CLASS_AFTER}"
log "After — TLS secretName: ${TLS_AFTER}"
log "After — cert-manager annotation: ${ISSUER_AFTER}"

assert_eq "Ingress UID unchanged" "$UID_BEFORE" "$UID_AFTER" || fail=1
assert_eq "ingress host" "pot-v2.local" "$HOST_AFTER" || fail=1
assert_eq "ingressClassName" "traefik" "$CLASS_AFTER" || fail=1
assert_eq "TLS secretName" "pot-tls" "$TLS_AFTER" || fail=1
assert_eq "cert-manager annotation" "local-test" "$ISSUER_AFTER" || fail=1
[[ "$GEN_AFTER" -gt "$GEN_BEFORE" ]] && log "PASS ControlPlane generation increased" || { log "FAIL ControlPlane generation did not increase"; fail=1; }

if [[ "$fail" -ne 0 ]]; then
  die "Scenario B failed"
fi
log "Scenario B passed."
