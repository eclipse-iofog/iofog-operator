# Build the manager binary
FROM golang:1.13-alpine as builder

WORKDIR /operator

RUN apk add --update --no-cache bash curl git make

COPY ./go.* ./
COPY ./main.go ./
COPY ./Makefile ./
COPY ./apis/ ./apis/
COPY ./internal/ ./internal/
COPY ./controllers/ ./controllers/

RUN make build
RUN cp ./bin/iofog-operator /bin

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /operator/iofog-operator .
USER nonroot:nonroot

ENTRYPOINT ["/bin/iofog-operator"]
