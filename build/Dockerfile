FROM golang:1.12-alpine as backend

COPY ./cmd /go/src/github.com/eclipse-iofog/iofog-operator/cmd
COPY ./pkg /go/src/github.com/eclipse-iofog/iofog-operator/pkg
COPY ./Gopkg.* /go/src/github.com/eclipse-iofog/iofog-operator/
COPY ./Makefile /go/src/github.com/eclipse-iofog/iofog-operator/
COPY ./script /go/src/github.com/eclipse-iofog/iofog-operator/script
COPY ./.git /go/src/github.com/eclipse-iofog/iofog-operator/.git
COPY ./build /go/src/github.com/eclipse-iofog/iofog-operator/build
COPY ./deploy /go/src/github.com/eclipse-iofog/iofog-operator/deploy

WORKDIR /go/src/github.com/eclipse-iofog/iofog-operator

RUN apk add --update --no-cache bash curl git make

RUN script/bootstrap.sh

RUN make vendor 
RUN make build

FROM alpine:3.7
COPY --from=backend /go/src/github.com/eclipse-iofog/iofog-operator/bin /bin

ENTRYPOINT ["/bin/iofog-operator"]