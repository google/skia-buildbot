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


COPY . /

USER skia

ENTRYPOINT ["/usr/local/bin/fiddler_restart.sh"]
CMD ["--logtostderr", "--checkout=/tmp/skia/skia/", "--fiddle_root=/tmp", "--port=:8000"]

