FROM golang:1.12-alpine as backend

COPY . /go/src/github.com/eclipse-iofog/iofog-operator
WORKDIR /go/src/github.com/eclipse-iofog/iofog-operator
RUN apk add --update --no-cache bash curl git make && \
    make vendor && \
    make build

FROM alpine:3.7
COPY --from=backend /go/src/github.com/eclipse-iofog/iofog-operator/bin /bin

ENTRYPOINT ["/bin/iofog-operator"]