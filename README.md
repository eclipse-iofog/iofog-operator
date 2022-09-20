# ioFog Operator

Operator is a component of the ioFog Kubernetes Control Plane. It is responsible for consuming
Control Plane CRDs for the purposes of deploying ioFog Control Planes to Kubernetes clusters.

## Build from Source

Go 1.17.9+ is a prerequisite.

See all `make` commands by running:

```
make help
```

To build, go ahead and run:

```
make build
```

Note that the Makefile targets have a number of tooling dependencies. These are
installed automatically if not present (as the [Azure build/test pipeline](azure-pipelines.yml) requires them),
but for local development, you can use your own install method for the tools. They are:

- [controller-gen](https://github.com/kubernetes-sigs/controller-tools): k8s component
- [kustomize](https://kustomize.io): k8s component
- [golangci-lint](https://golangci-lint.run): linting
- [kubectl](https://kubectl.docs.kubernetes.io): CLI for controlling k8s
- [bats](https://github.com/bats-core/bats-core): Bash-based testing framework


## Running Tests

Run project unit tests:

```
make test
```

## Running off-cluster

```
export KUBECONFIG=~/.kube/config
bin/iofog-operator
```
