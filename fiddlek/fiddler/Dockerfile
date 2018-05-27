FROM gcr.io/skia-public/skia-release:prod

USER root

RUN apt-get update && apt-get upgrade -y && apt-get install -y  \
  libfontconfig1 \
  libglu1-mesa \
  ffmpeg \
  xvfb \
  && rm -rf /var/lib/apt/lists/*

ENTRYPOINT ["/usr/local/bin/fiddler"]
CMD ["--logtostderr"]

