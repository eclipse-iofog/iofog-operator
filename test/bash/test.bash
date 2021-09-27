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
    'Starting Controller	{"reconcilerGroup": "iofog.org", "reconcilerKind": "Application", "controller": "application"}'
    'Starting workers	{"reconcilerGroup": "iofog.org", "reconcilerKind": "Application", "controller": "application", "worker count": 1}'
    'Starting Controller	{"reconcilerGroup": "iofog.org", "reconcilerKind": "ControlPlane", "controller": "controlplane"}'
    'Starting workers	{"reconcilerGroup": "iofog.org", "reconcilerKind": "ControlPlane", "controller": "controlplane", "worker count": 1}'
  )
  for TXT in "${TXTS[@]}"; do
    waitCmdGrep 30 "kctl logs -l name=iofog-operator" "$TXT"
  done
  stopTest
}

function testCreateControlplane() {
  startTest
  kctl apply -f config/cr/controlplane.yaml
  local TXTS=(
    "Successfully Reconciled	{\"reconcilerGroup\": \"iofog.org\", \"reconcilerKind\": \"ControlPlane\", \"controller\": \"controlplane\", \"name\": \"iofog\", \"namespace\": \"$NAMESPACE\"}"
  )
  for TXT in "${TXTS[@]}"; do
    waitCmdGrep 180 "kctl logs -l name=iofog-operator" "$TXT"
  done
  kctl wait --for=condition=Ready pods -l name=controller --timeout 1m
  kctl wait --for=condition=Ready pods -l name=port-manager --timeout 1m
  kctl wait --for=condition=Ready pods -l name=router --timeout 1m
  [ -z "$(kctl logs -l name=iofog-operator | grep "ERROR")" ]
  stopTest
}
