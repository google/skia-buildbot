FROM debian:testing-slim

RUN apt-get update && apt-get upgrade -y && apt-get install -y  \
  libfontconfig1 \
  libglu1-mesa \
  xvfb \
  ca-certificates \
  && rm -rf /var/lib/apt/lists/* \
  && groupadd -g 2000 skia \
  && useradd -u 2000 -g 2000 skia

USER skia

ADD --chown=skia:skia https://storage.googleapis.com/swiftshader-binaries/OpenGL_ES/Latest/Linux/libGLESv2.so /usr/local/lib/libGLESv2.so
ADD --chown=skia:skia https://storage.googleapis.com/swiftshader-binaries/OpenGL_ES/Latest/Linux/libEGL.so /usr/local/lib/libEGL.so
COPY . /
COPY --from=gcr.io/skia-public/skia-release:prod /tmp/skia/skia/out/Static/skiaserve /usr/local/bin/skiaserve
COPY --from=gcr.io/skia-public/skia-release:prod /tmp/skia/skia/VERSION /etc/skia-prod/VERSION


ENTRYPOINT ["/usr/local/bin/debugger"]
CMD ["--logtostderr", "--resources_dir=/usr/local/share/debugger"]
