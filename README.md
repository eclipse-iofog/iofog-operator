# ioFog Operator

Operator is a component of the ioFog Kubernetes Control Plane. It is responsible for consuming Control Plane CRDs for the purposes of deploying ioFog Control Planes to Kubernetes clusters.

## Build from Source

Go 1.15+ is a prerequisite.

See all `make` commands by running:
```
make help
```

To build, go ahead and run:
```
make build
```

## Running Tests

Run project unit tests:
```
make test
```