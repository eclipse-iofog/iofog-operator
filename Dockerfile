FROM golang:1.19-alpine as builder

WORKDIR /operator

RUN apk add --update --no-cache bash curl git make

COPY ./go.* ./
COPY ./Makefile ./
RUN make controller-gen

COPY ./main.go ./
COPY ./apis/ ./apis/
COPY ./internal/ ./internal/
COPY ./controllers/ ./controllers/
COPY ./hack/ ./hack/

RUN make build
RUN cp ./bin/iofog-operator /bin

FROM alpine:3.16
WORKDIR /
COPY --from=builder /bin/iofog-operator /bin/

ENTRYPOINT ["/bin/iofog-operator", "--enable-leader-election"]
