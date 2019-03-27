FROM golang:1.12-alpine as backend
RUN apk add --update --no-cache bash curl git make

RUN mkdir -p /go/src/github.com/iofog/iofog-operator
ADD Gopkg.* Makefile /go/src/github.com/iofog/iofog-operator/
WORKDIR /go/src/github.com/iofog/iofog-operator
RUN make vendor
ADD . /go/src/github.com/iofog/iofog-operator

RUN make build

FROM alpine:3.7
COPY --from=backend /go/src/github.com/iofog/iofog-operator/bin /bin

ENTRYPOINT ["/bin/iofog-operator"]