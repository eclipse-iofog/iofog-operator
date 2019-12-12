FROM golang:1.12-alpine as backend

COPY ./go.* /
COPY ./cmd /cmd
COPY ./pkg /pkg
COPY ./Makefile /
COPY ./build /build
COPY ./script /script
COPY ./vendor /vendor

WORKDIR /

RUN apk add --update --no-cache bash curl git make

RUN ./script/bootstrap.sh
RUN make build

FROM alpine:3.7
COPY --from=backend /bin /bin

ENTRYPOINT ["/bin/iofog-operator"]
