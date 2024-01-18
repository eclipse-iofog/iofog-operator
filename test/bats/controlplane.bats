#!/usr/bin/env bash

. test/bash/include.bash

@test "Initialize tests" {
    stopTest
}

@test "Verify kubectl works" {
    testKubectl
}

@test "Create k8s namespace" {
    testCreateNamespace
}

@test "Create crds" {
    testCreateCRD
}

@test "Deploy operator" {
    testDeployOperator
}

@test "Create controlplane" {
    testCreateControlplane
}

#
#@test "Delete k8s namespace" {
#    testDeleteNamespace
#}