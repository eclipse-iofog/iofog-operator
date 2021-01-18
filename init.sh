#!/bin/sh

# controller-gen v0.3.0
brew install operator-sdk # v1.3.0
brew install kubebuilder # v2.3.1
operator-sdk init --domain=iofog.org --repo=github.com/eclipse-iofog/iofog-operator --plugins go.kubebuilder.io/v2
kubebuilder edit --multigroup=true

# NOTE: groups were manually removed after generation
operator-sdk create api --group apps --version v2 --kind Application --resource=true --controller=true
operator-sdk create api --group controlplanes --version v2 --kind ControlPlane --resource=true --controller=true