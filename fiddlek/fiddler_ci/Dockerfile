# Keep this in sync with ../fiddler/Dockerfile and ../build_fiddler_release.
FROM golang:1.10.3-stretch as build

RUN go get go.skia.org/infra/fiddlek/go/fiddler go.skia.org/infra/fiddlek/go/smoketest \
    && /go/bin/fiddler --logtostderr --checkout=/tmp/skia/skia/ --fiddle_root=/tmp --port=:8000 & \
    && sleep 2  \
    && /go/bin/smoketest

FROM gcr.io/skia-public/skia-release:prod

USER root

RUN apt-get update && apt-get upgrade -y && apt-get install -y  \
  libfontconfig1 \
  libglu1-mesa \
  ffmpeg \
  xvfb \
  && rm -rf /var/lib/apt/lists/* \
  && useradd you-are-still \
  && useradd in-a \
  && useradd container

COPY --from=build --chown=2000:2000 /go/bin/fiddler /usr/local/bin/fiddler
COPY --from=build --chown=2000:2000 /go/src/go.skia.org/infra/fiddlek/source/* /etc/fiddle/source/

USER skia

ENTRYPOINT ["/usr/local/bin/fiddler"]
CMD ["--logtostderr", "--checkout=/tmp/skia/skia/", "--fiddle_root=/tmp", "--port=:8000"]
