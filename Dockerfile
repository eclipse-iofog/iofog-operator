FROM golang:1.26.4-alpine3.23@sha256:f23e8b227fb4493eabe03bede4d5a32d04092da71962f1fb79b5f7d1e6c2a17f AS builder

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

FROM registry.access.redhat.com/ubi9/ubi-minimal:9.6@sha256:34880b64c07f28f64d95737f82f891516de9a3b43583f39970f7bf8e4cfa48b7
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
