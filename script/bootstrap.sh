#!/bin/sh

OS=$(uname -s | tr A-Z a-z)

if [ "$OS" == "darwin" ]; then
    OS=apple-darwin
elif [ "$OS" == "linux" ]; then
    OS=linux-gnu
fi

RELEASE_VERSION=v0.10.0

curl -OJL https://github.com/operator-framework/operator-sdk/releases/download/"$RELEASE_VERSION"/operator-sdk-"$RELEASE_VERSION"-x86_64-"$OS"
chmod +x operator-sdk-"$RELEASE_VERSION"-x86_64-"$OS"
mv operator-sdk-"$RELEASE_VERSION"-x86_64-"$OS" /usr/local/bin/operator-sdk

# Golang
go get k8s.io/gengo/args
go get k8s.io/code-generator
go get k8s.io/apimachinery