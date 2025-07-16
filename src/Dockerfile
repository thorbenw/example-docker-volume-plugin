FROM golang:1.23.0-alpine AS builder
ARG MODULE_VERSION="(docker)"
COPY . /go/src/github.com/keebits/example-docker-volume-plugin
WORKDIR /go/src/github.com/keebits/example-docker-volume-plugin
# Always link statically: https://stackoverflow.com/questions/66963068/docker-alpine-executable-binary-not-found-even-if-in-path
RUN find . -type f \
    && go mod tidy \
    && set -ex \
    && apk add --no-cache --virtual .build-deps \
    gcc libc-dev \
    && go install --ldflags "-extldflags '-static' -X 'main.version=${MODULE_VERSION}'" \
    && gcc examplemount/examplemount.c -o /go/bin/examplemount -static -fno-builtin -D DEBUG \
    && apk del .build-deps
CMD ["/go/bin/example-docker-volume-plugin"]

FROM alpine:3.22.0
RUN apk update
#RUN apk update && apk add sshfs
COPY --from=builder /go/bin/example-docker-volume-plugin .
COPY --from=builder /go/bin/examplemount .
CMD ["/example-docker-volume-plugin"]
