#!/usr/bin/env bash

. test/bash/alias.bash

function testKubectl() {
  startTest
  kctl get ns
  stopTest
}

function testDeployOperator() {
    startTest
    kctl apply -f config/operator/
    kctl wait --for=condition=Ready pod/iofog-operator
    kctl logs -l name=iofog-operator | grep "INFO	setup	starting manager"
    kctl logs -l name=iofog-operator | grep "successfully acquired lease"
    kctl logs -l name=iofog-operator | grep 'Starting Controller	{"reconcilerGroup": "iofog.org", "reconcilerKind": "Application", "controller": "application"}'
    kctl logs -l name=iofog-operator | grep 'Starting workers	{"reconcilerGroup": "iofog.org", "reconcilerKind": "Application", "controller": "application", "worker count": 1}'
    kctl logs -l name=iofog-operator | grep 'Starting Controller	{"reconcilerGroup": "iofog.org", "reconcilerKind": "ControlPlane", "controller": "controlplane"}'
    kctl logs -l name=iofog-operator | grep 'Starting workers	{"reconcilerGroup": "iofog.org", "reconcilerKind": "ControlPlane", "controller": "controlplane", "worker count": 1}'
    stopTest
}
