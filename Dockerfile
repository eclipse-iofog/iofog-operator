FROM golang:1.26.6-alpine3.23@sha256:e57c41c1d5864341031181b0db34b9a537bb5773eb6428e4e5bdaea0f9135406 AS builder

WORKDIR /operator

RUN apk add --update --no-cache bash curl git make

ARG TARGETOS
ARG TARGETARCH
ARG IMAGE_REGISTRY=ghcr.io/eclipse-iofog
ARG OPERATOR_COMPONENT_LABEL_DOMAIN=iofog.org

ENV GOOS=$TARGETOS \
    GOARCH=$TARGETARCH \
    IMAGE_REGISTRY=$IMAGE_REGISTRY \
    OPERATOR_COMPONENT_LABEL_DOMAIN=$OPERATOR_COMPONENT_LABEL_DOMAIN

COPY ./go.* ./
COPY ./Makefile ./
RUN make controller-gen

COPY ./main.go ./
COPY ./apis/ ./apis/
COPY ./internal/ ./internal/
COPY ./controllers/ ./controllers/
COPY ./pkg/ ./pkg/
COPY ./hack/ ./hack/

RUN make build
RUN cp ./bin/iofog-operator /bin

FROM registry.access.redhat.com/ubi9/ubi-minimal:9.8@sha256:8eb2830d0936237fc13a1f2f7e45aecf90d69043380ad167fad0343632937f41
WORKDIR /

ARG OCI_SOURCE_REPO=https://github.com/eclipse-iofog/iofog-operator
ARG IMAGE_REGISTRY=ghcr.io/eclipse-iofog

RUN microdnf install -y shadow-utils && \
    microdnf clean all
RUN useradd --uid 10000 runner


COPY LICENSE /licenses/LICENSE
COPY --from=builder /bin/iofog-operator /bin/
LABEL org.opencontainers.image.description=operator
LABEL org.opencontainers.image.source=${OCI_SOURCE_REPO}
LABEL org.opencontainers.image.url=${IMAGE_REGISTRY}/operator
LABEL org.opencontainers.image.licenses=EPL2.0

USER 10000

ENTRYPOINT ["/bin/iofog-operator", "--enable-leader-election"]
