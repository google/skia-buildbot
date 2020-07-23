
# Args do not yet expand in COPY: https://github.com/docker/cli/issues/996
# So using workaround with Args mentioned in the above issue.
ARG SKIA_IMAGE_NAME=skia-release
ARG SKIA_IMAGE_TAG=prod
FROM gcr.io/skia-public/${SKIA_IMAGE_NAME}:${SKIA_IMAGE_TAG} as SKIA_IMAGE

FROM debian:testing
# FROM debian:testing-slim slim broke early Sept 2018.

RUN apt-get update && apt-get upgrade -y && apt-get install -y  \
  libfontconfig1 \
  libglu1-mesa \
  xvfb \
  ca-certificates \
  && rm -rf /var/lib/apt/lists/* \
  && groupadd -g 2000 skia \
  && useradd -u 2000 -g 2000 skia

USER skia

ADD --chown=skia:skia https://storage.googleapis.com/skia-swiftshader/libGLESv2.so /usr/local/lib/libGLESv2.so
ADD --chown=skia:skia https://storage.googleapis.com/skia-swiftshader/libEGL.so /usr/local/lib/libEGL.so
COPY . /
COPY --from=SKIA_IMAGE /tmp/skia/skia/out/Static/skiaserve /usr/local/bin/skiaserve
COPY --from=SKIA_IMAGE /tmp/skia/skia/VERSION /etc/skia-prod/VERSION


ENTRYPOINT ["/usr/local/bin/debugger"]
CMD ["--logtostderr", "--resources_dir=/usr/local/share/debugger"]
