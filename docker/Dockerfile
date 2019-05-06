# Dockerfile for Skia Infrastructure CI environment.
FROM debian:testing
RUN groupadd -g 2000 skia \
  && useradd -m -u 2000 -g 2000 skia \
  && apt-get update && apt-get upgrade -y && apt-get install -y \
  build-essential \
  git \
  curl \
  clang \
  wget

USER skia

RUN mkdir -p /home/skia/golib \
    && cd /home/skia \
    && wget https://dl.google.com/go/go1.12.4.linux-amd64.tar.gz \
    && tar -xf go1.12.4.linux-amd64.tar.gz

ENV GOPATH /home/skia/golib
ENV PATH $GOPATH/bin:/home/skia/go/bin:$PATH
ENV GO111MODULE on

RUN cd /home/skia && git clone https://skia.googlesource.com/buildbot.git
RUN cd /home/skia/buildbot && go get
RUN cd /home/skia/buildbot && \
  go get github.com/kisielk/errcheck \
  golang.org/x/tools/cmd/goimports \
  go.chromium.org/luci/client/cmd/isolate
RUN cd /home/skia/buildbot/fiddlek && make fiddle_secwrap

USER root

RUN curl -sL https://deb.nodesource.com/setup_10.x | bash - \
  && apt-get install -y nodejs

# USER skia
#
# RUN cd /home/skia/golib/src/go.skia.org/infra/perf \
#     && make
