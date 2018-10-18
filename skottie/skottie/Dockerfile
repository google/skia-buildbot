FROM debian:stretch
# FROM debian:testing-slim slim broke early Sept 2018.

RUN apt-get update && apt-get upgrade -y && apt-get install -y  \
  libfontconfig1 \
  libglu1-mesa \
  ffmpeg \
  xvfb \
  ca-certificates \
  && rm -rf /var/lib/apt/lists/* \
  && groupadd -g 2000 skia \
  && useradd -u 2000 -g 2000 skia

USER skia

ADD --chown=skia:skia https://storage.googleapis.com/swiftshader-binaries/OpenGL_ES/Latest/Linux/libGLESv2.so /usr/local/lib/libGLESv2.so
ADD --chown=skia:skia https://storage.googleapis.com/swiftshader-binaries/OpenGL_ES/Latest/Linux/libEGL.so /usr/local/lib/libEGL.so
COPY . /
COPY --from=gcr.io/skia-public/skia-release:prod /tmp/skia/skia/out/Static/skottie_tool /usr/local/bin/skottie_tool
COPY --from=gcr.io/skia-public/skia-release:prod /tmp/skia/skia/VERSION /etc/skia-prod/VERSION

ENTRYPOINT ["/usr/local/bin/skottie"]
CMD ["--logtostderr", "--resources_dir=/usr/local/share/skottie", "--skottie_tool=/usr/local/bin/skottie_tool"]
