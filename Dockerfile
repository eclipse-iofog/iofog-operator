FROM golang:1.15-alpine as builder

WORKDIR /operator

RUN apk add --update --no-cache bash curl git make

COPY ./go.* ./
COPY ./vendor/ ./vendor/
COPY ./Makefile ./
RUN make controller-gen

COPY ./main.go ./
COPY ./apis/ ./apis/
COPY ./internal/ ./internal/
COPY ./controllers/ ./controllers/
COPY ./hack/ ./hack/

RUN make build
RUN cp ./bin/iofog-operator /bin

FROM alpine:3.7
WORKDIR /
COPY --from=builder /bin/iofog-operator /

ENTRYPOINT ["/iofog-operator"]
