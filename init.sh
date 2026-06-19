#!/bin/sh

# controller-gen v0.3.0
brew install operator-sdk # v1.3.0
brew install kubebuilder # v2.3.1
operator-sdk init --domain=datasance.com --repo=github.com/datasance/iofog-operator --plugins go.kubebuilder.io/v4
kubebuilder edit --multigroup=true

# NOTE: groups were manually removed after generation
operator-sdk create api --group apps --version v3 --kind Application --resource=true --controller=true
operator-sdk create api --group controlplanes --version v3 --kind ControlPlane --resource=true --controller=true