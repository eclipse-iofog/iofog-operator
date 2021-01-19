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

function testDeployOperator() {
    startTest
    kctl apply -f config/operator/
    kctl wait --for=condition=available deployments iofog-operator -n default --timeout 1m
    kctl logs -l name=iofog-operator | grep "INFO	setup	starting manager"
    kctl logs -l name=iofog-operator | grep "successfully acquired lease"
    kctl logs -l name=iofog-operator | grep 'Starting Controller	{"reconcilerGroup": "iofog.org", "reconcilerKind": "Application", "controller": "application"}'
    kctl logs -l name=iofog-operator | grep 'Starting workers	{"reconcilerGroup": "iofog.org", "reconcilerKind": "Application", "controller": "application", "worker count": 1}'
    kctl logs -l name=iofog-operator | grep 'Starting Controller	{"reconcilerGroup": "iofog.org", "reconcilerKind": "ControlPlane", "controller": "controlplane"}'
    kctl logs -l name=iofog-operator | grep 'Starting workers	{"reconcilerGroup": "iofog.org", "reconcilerKind": "ControlPlane", "controller": "controlplane", "worker count": 1}'
    stopTest
}
