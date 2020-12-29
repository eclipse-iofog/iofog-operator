FROM golang:1.13-alpine as backend

WORKDIR /operator

RUN apk add --update --no-cache bash curl git make

COPY ./go.* ./
COPY ./vendor ./vendor
RUN go install -mod=vendor k8s.io/gengo/examples/deepcopy-gen/

COPY ./cmd ./cmd
COPY ./internal ./internal
COPY ./pkg ./pkg
COPY ./Makefile ./

RUN make build
RUN cp ./bin/iofog-operator /bin

FROM alpine:3.7
COPY --from=backend /bin /bin

ENTRYPOINT ["/bin/iofog-operator"]
