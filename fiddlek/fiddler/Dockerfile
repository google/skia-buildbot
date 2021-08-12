ARG SKIA_IMAGE_NAME=skia-release
ARG SKIA_IMAGE_TAG=prod
FROM gcr.io/skia-public/${SKIA_IMAGE_NAME}:${SKIA_IMAGE_TAG}

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


COPY . /

USER skia

ENTRYPOINT ["/usr/local/bin/fiddler"]
CMD ["--checkout=/tmp/skia/skia/", "--fiddle_root=/tmp", "--port=:8000", "--local"]

