#!/usr/bin/env bash
# Shared helpers for local E2E scripts (Plan 5-4).

set -euo pipefail

TEST_NAMESPACE="${TEST_NAMESPACE:-iofog-test}"
KUBECTL="${KUBECTL:-kubectl}"
CONTROLPLANE_NAME="${CONTROLPLANE_NAME:-iofog}"
RECONCILE_WAIT_SECS="${RECONCILE_WAIT_SECS:-15}"
BASELINE_WAIT_SECS="${BASELINE_WAIT_SECS:-600}"

log() {
  printf '[local-e2e] %s\n' "$*"
}

die() {
  printf '[local-e2e] ERROR: %s\n' "$*" >&2
  exit 1
}

require_kubectl() {
  command -v "$KUBECTL" >/dev/null 2>&1 || die "kubectl not found (set KUBECTL=...)"
  "$KUBECTL" cluster-info >/dev/null 2>&1 || die "cluster not reachable (check KUBECONFIG)"
}

wait_postgres() {
  log "Waiting for postgres Deployment in ${TEST_NAMESPACE}..."
  "$KUBECTL" wait -n "$TEST_NAMESPACE" --for=condition=Available deploy/postgres --timeout=120s
}

wait_baseline_resources() {
  log "Waiting for operator-created baseline resources in ${TEST_NAMESPACE} (timeout ${BASELINE_WAIT_SECS}s)..."
  local ctrl_type=""
  local deadline=$((SECONDS + BASELINE_WAIT_SECS))
  while (( SECONDS < deadline )); do
    ctrl_type="$("$KUBECTL" get controlplane "$CONTROLPLANE_NAME" -n "$TEST_NAMESPACE" \
      -o jsonpath='{.spec.services.controller.type}' 2>/dev/null || true)"
    local has_ingress_check=true
    if [[ "$(echo "$ctrl_type" | tr '[:upper:]' '[:lower:]')" == "loadbalancer" ]]; then
      has_ingress_check=false
    fi
    local ingress_ok=true
    if $has_ingress_check; then
      if ! "$KUBECTL" get ingress controller -n "$TEST_NAMESPACE" >/dev/null 2>&1; then
        ingress_ok=false
      fi
    fi
    if "$KUBECTL" get svc controller -n "$TEST_NAMESPACE" >/dev/null 2>&1 \
      && $ingress_ok \
      && "$KUBECTL" get deploy controller -n "$TEST_NAMESPACE" >/dev/null 2>&1 \
      && "$KUBECTL" get secret controller-auth-credentials -n "$TEST_NAMESPACE" >/dev/null 2>&1; then
      log "Baseline resources present (controller Service, controller Deployment, auth secret)."
      wait_controlplane_ready
      return 0
    fi
    sleep 5
  done
  die "Timed out waiting for baseline resources. Is the operator running (make run or local-deploy-operator)?"
}

wait_controlplane_ready() {
  log "Waiting for ControlPlane ${CONTROLPLANE_NAME} Ready (timeout ${BASELINE_WAIT_SECS}s)..."
  local deadline=$((SECONDS + BASELINE_WAIT_SECS))
  while (( SECONDS < deadline )); do
    local cond gen obs
    cond="$("$KUBECTL" get controlplane "$CONTROLPLANE_NAME" -n "$TEST_NAMESPACE" \
      -o jsonpath='{.status.conditions[?(@.status=="True")].type}' 2>/dev/null || true)"
    gen="$("$KUBECTL" get controlplane "$CONTROLPLANE_NAME" -n "$TEST_NAMESPACE" -o jsonpath='{.metadata.generation}' 2>/dev/null || true)"
    obs="$("$KUBECTL" get controlplane "$CONTROLPLANE_NAME" -n "$TEST_NAMESPACE" \
      -o jsonpath='{.status.conditions[?(@.status=="True")].observedGeneration}' 2>/dev/null || true)"
    if [[ "$cond" == "ready" && "$gen" == "$obs" ]]; then
      log "ControlPlane Ready (generation ${gen})."
      return 0
    fi
    sleep 10
  done
  log "WARN: ControlPlane not Ready within timeout; patch scenarios may be flaky."
}

wait_reconcile() {
  log "Waiting ${RECONCILE_WAIT_SECS}s for reconcile..."
  sleep "$RECONCILE_WAIT_SECS"
}

wait_for_patch() {
  local check_cmd=$1
  local label=$2
  local timeout=${3:-120}
  local deadline=$((SECONDS + timeout))
  while (( SECONDS < deadline )); do
    if eval "$check_cmd"; then
      log "PASS ${label} (patch applied)"
      return 0
    fi
    sleep 3
  done
  log "FAIL ${label} (patch not applied within ${timeout}s)"
  return 1
}

cp_generation() {
  "$KUBECTL" get controlplane "$CONTROLPLANE_NAME" -n "$TEST_NAMESPACE" -o jsonpath='{.metadata.generation}'
}

auth_password() {
  "$KUBECTL" get secret controller-auth-credentials -n "$TEST_NAMESPACE" \
    -o jsonpath='{.data.auth-bootstrap-password}' | base64 -d
}

assert_eq() {
  local label="$1"
  local want="$2"
  local got="$3"
  if [[ "$want" != "$got" ]]; then
    printf '[local-e2e] FAIL %s\n  want: %q\n  got:  %q\n' "$label" "$want" "$got" >&2
    return 1
  fi
  printf '[local-e2e] PASS %s\n' "$label"
}

assert_contains() {
  local label="$1"
  local haystack="$2"
  local needle="$3"
  if [[ "$haystack" != *"$needle"* ]]; then
    printf '[local-e2e] FAIL %s — output does not contain %q\n%s\n' "$label" "$needle" "$haystack" >&2
    return 1
  fi
  printf '[local-e2e] PASS %s\n' "$label"
}

print_header() {
  echo ""
  echo "========================================"
  echo "$*"
  echo "========================================"
}
