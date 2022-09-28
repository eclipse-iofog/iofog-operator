#!/bin/bash

set -e

echo "kubectl get namespaces"
kubectl get namespaces


echo "gke-gcloud-auth-plugin: $(gke-gcloud-auth-plugin version)"
gke-gcloud-auth-plugin --version

echo "done"

# Export variables
CONF=test/conf/env.sh
if [ -f "$CONF" ]; then
    . "$CONF"
fi

# Get test names from args, run all if empty
TESTS="$1"
if [ -z "$TESTS" ]; then
    TESTS=("controlplane")
fi

# Run tests
for TEST in ${TESTS[@]}; do
    bats "test/bats/${TEST}.bats"
done
