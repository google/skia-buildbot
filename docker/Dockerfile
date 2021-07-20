# Dockerfile for Skia Infrastructure CI environment.

# TODO(jcgregorio) Switch back to debian:testing after
# https://github.com/nodesource/distributions/issues/873 is resolved.
FROM debian:buster
RUN groupadd -g 2000 skia \
  && useradd -m -u 2000 -g 2000 skia \
  && apt-get update && apt-get upgrade -y && apt-get install -y \
  build-essential \
  git \
  curl \
  clang \
  nodejs \
  wget

USER skia

RUN mkdir -p /home/skia/golib/src \
    && cd /home/skia \
    && wget https://dl.google.com/go/go1.16.3.linux-amd64.tar.gz \
    && tar -xf go1.16.3.linux-amd64.tar.gz

ENV GOPATH /home/skia/golib
ENV PATH $GOPATH/bin:/home/skia/go/bin:$PATH
ENV GO111MODULE on

RUN cd $GOPATH/src \
  && mkdir go.skia.org \
  && cd go.skia.org \
  && git clone https://skia.googlesource.com/buildbot.git infra

# Set fake identity for git rebase. See thread in
# https://skia-review.googlesource.com/c/buildbot/+/286537/5/docker/Dockerfile#46
RUN cd $GOPATH/src/go.skia.org/infra \
    && git config user.email "skia@skia.org" \
    && git config user.name "Skia"

# HASH must be specified.
ARG HASH
RUN if [ -z "${HASH}" ] ; then echo "HASH must be specified as a --build-arg"; exit 1; fi

RUN cd $GOPATH/src/go.skia.org/infra \
  && git fetch \
  && git reset --hard ${HASH}

# If patch ref is specified then update the ref to patch in a CL.
ARG PATCH_REF
RUN if [ ! -z "${PATCH_REF}" ] ; then cd $GOPATH/src/go.skia.org/infra \
    && git fetch https://skia.googlesource.com/buildbot ${PATCH_REF} \
    && git checkout FETCH_HEAD \
    && git rebase ${HASH}; fi

RUN cd $GOPATH/src/go.skia.org/infra \
    && go install ./...

RUN cd $GOPATH/src/go.skia.org/infra/fiddlek \
  && make fiddle_secwrap

USER root

# USER skia
#
# RUN cd /home/skia/golib/src/go.skia.org/infra/perf \
#     && make
