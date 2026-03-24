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
  stopTest
  kctl delete ns "$NAMESPACE"
}

function testCreateCRD() {
  startTest
  kctl apply -f config/crd/bases/iofog.org_applications.yaml
  kctl apply -f config/crd/bases/iofog.org_controlplanes.yaml
  kctl get crds | grep "controlplanes\.iofog\.org"
  kctl get crds | grep "apps\.iofog\.org"
  stopTest
}

function testDeployOperator() {
  startTest
  kctl apply -f config/operator/rbac.yaml
  kctl apply -f config/operator/deployment.yaml
  kctl wait --for=condition=Ready pods -l name=iofog-operator --timeout 1m
  kctl describe pods -l name=iofog-operator | grep "$OP_VERSION"
  local TXTS=(
    "successfully acquired lease"
    'Starting Controller	{"controller": "application", "controllerGroup": "iofog.org", "controllerKind": "Application"}'
    'Starting Controller	{"controller": "controlplane", "controllerGroup": "iofog.org", "controllerKind": "ControlPlane"}'
    'Starting workers	{"controller": "application", "controllerGroup": "iofog.org", "controllerKind": "Application", "worker count": 1}'
    'Starting workers	{"controller": "controlplane", "controllerGroup": "iofog.org", "controllerKind": "ControlPlane", "worker count": 1}'
  )
  for TXT in "${TXTS[@]}"; do
    waitCmdGrep 30 "kctl logs -l name=iofog-operator" "$TXT"
  done
  stopTest
}

function testCreateControlplane() {
  startTest
  kctl apply -f config/cr/controlplane.yaml
  waitCmdGrep 180 "kctl get controlplane iofog -oyaml" "type: ready"
  # Pod will restart once, so wait for either of them to be ready
  kctl wait --for=condition=Ready pods -l name=controller --timeout 1m || kctl wait --for=condition=Ready pods -l name=controller --timeout 1m
  kctl wait --for=condition=Ready pods -l name=port-manager --timeout 1m
  kctl wait --for=condition=Ready pods -l name=router --timeout 1m
  [ -z "$(kctl logs -l name=iofog-operator | grep "ERROR")" ]
  stopTest
}
