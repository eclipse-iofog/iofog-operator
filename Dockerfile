FROM golang:1.13-alpine as backend

WORKDIR /operator

RUN apk add --update --no-cache bash curl git make

COPY ./go.* ./
COPY ./vendor ./vendor
COPY ./script ./script
RUN ./script/bootstrap.sh

COPY ./cmd ./cmd
COPY ./internal ./internal
COPY ./pkg ./pkg
COPY ./Makefile ./

RUN make build
RUN cp ./bin/iofog-operator /bin

FROM alpine:3.7
COPY --from=backend /bin /bin

ENTRYPOINT ["/bin/iofog-operator"]
