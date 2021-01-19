#!/usr/bin/env bash

function testKubectl() {
  startTest
  kctl get ns
  stopTest
}

function testCreateNamespace() {
  startTest
  kctl create ns "$NAMESPACE"
  stopTest
}

function testDeleteNamespace() {
  startTest
  kctl delete ns "$NAMESPACE"
  stopTest
}

function testCreateCRD() {
  startTest
  kctl apply -f config/crds/applications.yaml
  kctl apply -f config/crds/controlplanes.yaml
  kctl get crds | grep controlplane
  kctl get crds | grep application
  stopTest
}

function testDeployOperator() {
  startTest
  kctl apply -f config/operator/rbac.yaml
  kctl apply -f config/operator/deployment.yaml
  kctl wait --for=condition=Ready pods -l name=iofog-operator --timeout 1m

  local TIMEOUT=30
  local SECS=0
  local STATUS=1
  local TXTS=(
    "successfully acquired lease"
    'Starting Controller	{"reconcilerGroup": "iofog.org", "reconcilerKind": "Application", "controller": "application"}'
    'Starting workers	{"reconcilerGroup": "iofog.org", "reconcilerKind": "Application", "controller": "application", "worker count": 1}'
    'Starting Controller	{"reconcilerGroup": "iofog.org", "reconcilerKind": "ControlPlane", "controller": "controlplane"}'
    'Starting workers	{"reconcilerGroup": "iofog.org", "reconcilerKind": "ControlPlane", "controller": "controlplane", "worker count": 1}'
  )
  while [ "$SECS" -lt "$TIMEOUT" ] && [ "$STATUS" -ne 0 ]; do
    STATUS=0
    local LOGS=$(kctl logs -l name=iofog-operator)
    for TXT in "${TXTS[@]}"; do
      local FOUND=$(echo "$LOGS" | grep "$TXT")
      if [ -z "$FOUND" ]; then
        STATUS=1
      fi
      log $STATUS
    done
    let "SECS=SECS+1"
    sleep 1
  done
  [ "$STATUS" -eq 0 ]
  [ "$SECS" -lt "$TIMEOUT" ]
  stopTest
}
