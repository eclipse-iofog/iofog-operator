FROM alpine:3.8

RUN apk upgrade --update --no-cache

USER nobody

ADD build/_output/bin/iofog-operator /usr/local/bin/iofog-operator
