FROM golang:1.12-alpine as backend

WORKDIR /operator

COPY ./go.* ./
COPY ./cmd ./cmd
COPY ./internal ./internal
COPY ./pkg ./pkg
COPY ./Makefile ./
COPY ./build ./build
COPY ./script ./script
COPY ./vendor ./vendor

RUN apk add --update --no-cache bash curl git make

RUN ./script/bootstrap.sh
RUN make build
RUN cp ./bin/iofog-operator /bin

FROM alpine:3.7
COPY --from=backend /bin /bin

ENTRYPOINT ["/bin/iofog-operator"]
