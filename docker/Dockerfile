# Dockerfile for building Skia in release mode, using 3rd party libs from DEPS, with SwiftShader.
FROM debian:testing
RUN groupadd -g 2000 skia \
  && useradd -m -u 2000 -g 2000 skia \
  && apt-get update && apt-get upgrade -y && apt-get install -y \
  build-essential \
  git \
  wget

USER skia

RUN mkdir -p /home/skia/golib \
    && cd /home/skia \
    && wget https://dl.google.com/go/go1.10.3.linux-amd64.tar.gz \
    && tar -xf go1.10.3.linux-amd64.tar.gz

ENV GOPATH /home/skia/golib
ENV PATH $GOPATH/bin:/home/skia/go/bin:$PATH

RUN go get -u -t go.skia.org/infra/... \
  github.com/kisielk/errcheck \
  golang.org/x/tools/cmd/goimports \
  go.chromium.org/luci/client/cmd/isolate

