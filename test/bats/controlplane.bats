#!/usr/bin/env bash

. test/bash/include.bash

@test "Initialize tests" {
    stopTest
}

@test "Verify kubectl works" {
    testKubectl
}

@test "Deploy operator" {
    testDeployOperator
}
