FROM golang:1.23.0-alpine AS builder
ARG MODULE_VERSION="(docker)"
COPY . /go/src/github.com/thorbenw/example-docker-volume-plugin
WORKDIR /go/src/github.com/thorbenw/example-docker-volume-plugin
# Always link statically: https://stackoverflow.com/questions/66963068/docker-alpine-executable-binary-not-found-even-if-in-path
RUN set -ex \
    && find . -type f \
    && go install --ldflags "-extldflags '-static' -X 'main.version=${MODULE_VERSION}'"

# Use the same alpine image version that the above golang image is based on
FROM alpine:3.20.2
RUN apk add s3fs-fuse mailcap
COPY --from=builder /go/bin/example-docker-volume-plugin .
CMD ["/example-docker-volume-plugin"]
