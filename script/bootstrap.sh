#!/bin/sh

# Is gengo installed?
if [ -z $(command -v deepcopy-gen) ]; then
    echo " Attempting to install 'gengo'"
    go install -mod=vendor k8s.io/gengo/examples/deepcopy-gen/
fi

# Is go lint installed?
if [ ! "$(command -v golangci-lint)" ]; then
    if [ "$(uname -s)" = "Darwin" ]; then
        brew install golangci-lint
        brew upgrade golangci-lint
    else
        curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.33.0
    fi
fi
