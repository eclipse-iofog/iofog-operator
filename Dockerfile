FROM golang:1.26.5-alpine3.23@sha256:622e56dbc11a8cfe87cafa2331e9a201877271cbff918af53d3be315f3da88cc AS builder

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

FROM registry.access.redhat.com/ubi9/ubi-minimal:9.8@sha256:2e8edce823a48e51858f1fad3ff4cbf6875ce8a3f86b9eecf298bc2050c8652a
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
