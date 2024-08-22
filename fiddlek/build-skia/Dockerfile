# Dockerfile for building an image with all the tools needed to checkout and build Skia,
# as well as to run the fiddler (Fiddle Backend).

FROM launcher.gcr.io/google/debian11 AS build
RUN apt update && apt dist-upgrade -y && apt install -y \
  build-essential \
  clang \
  curl \
  ffmpeg \
  git \
  libfontconfig-dev \
  libfontconfig1 \
  libgl1-mesa-dev \
  libglu1-mesa \
  libglu1-mesa-dev \
  procps \
  python \
  xvfb \
  && rm -rf /var/lib/apt/lists/*

RUN groupadd -g 2000 skia \
  && useradd -u 2000 -g 2000 skia

RUN cd /tmp \
  && git clone --depth=1 'https://chromium.googlesource.com/chromium/tools/depot_tools.git'

ENV PATH=${PATH}:/tmp/depot_tools

ADD --chown=skia:skia https://storage.googleapis.com/skia-swiftshader/libGLESv2.so /usr/local/lib/libGLESv2.so
ADD --chown=skia:skia https://storage.googleapis.com/skia-swiftshader/libEGL.so /usr/local/lib/libEGL.so