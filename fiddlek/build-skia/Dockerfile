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
  nano \
  procps \
  python \
  xvfb \
  && rm -rf /var/lib/apt/lists/*

# Build the security wrapper
COPY . /tmp/

WORKDIR /tmp
RUN clang fiddle_secwrap.cpp -lstdc++ -o fiddle_secwrap

# compile test executables
RUN mkdir /tmp/tests
RUN clang success.cpp -lstdc++ -o ./tests/success
RUN clang fail_socket.cpp -o ./tests/fail_socket
RUN clang fail_execve.cpp -o ./tests/fail_execve
RUN clang fail_link.cpp -o ./tests/fail_link
RUN clang fail_rename.cpp -o ./tests/fail_rename
RUN clang fail_mknod.cpp -o ./tests/fail_mknod

# Make sure this binary runs on its own...
RUN ./tests/success
# ...and with the wrapper
RUN ./fiddle_secwrap ./tests/success

# If any of these don't work, the security wrapper is not blocking things correctly.
RUN ./fiddle_secwrap ./tests/fail_socket; test $? -eq 1
RUN ./fiddle_secwrap ./tests/fail_execve; test $? -eq 255
RUN ./fiddle_secwrap ./tests/fail_link; test $? -eq 255
RUN ./fiddle_secwrap ./tests/fail_rename; test $? -eq 255
RUN ./fiddle_secwrap ./tests/fail_mknod; test $? -eq 255

RUN mv ./fiddle_secwrap /usr/local/bin/fiddle_secwrap \
    && rm -r /tmp \
    && mkdir --mode=0777 /tmp

RUN groupadd -g 2000 skia \
  && useradd -u 2000 -g 2000 skia \
  && install -d -o skia -g skia -m 700 /home/skia

USER skia

# We need depot_tools to build Skia with GN. depot_tools doesn't like being in
# a file owned by root though, so we run these commands as the skia user.
RUN cd /tmp \
  && git clone --depth=1 'https://chromium.googlesource.com/chromium/tools/depot_tools.git'

ENV PATH=${PATH}:/tmp/depot_tools

# We build Skia against swiftshader so we can run fiddles against both the CPU and GPU backends.
# In particular, we need the headers in swiftshader/include (e.g. the xcb and vulkan ones).
# We delete some unnecessary and large folders to reduce our image size.
RUN cd /tmp \
  && git clone https://swiftshader.googlesource.com/SwiftShader swiftshader \
  && cd /tmp/swiftshader \
  && git checkout 3c4bdf66d81d01a215b88bfea3ac4cc8ca507779 \
  && rm -rf /tmp/swiftshader/.git \
           /tmp/swiftshader/tests \
           /tmp/swiftshader/third_party

# These are pre-compiled because Swiftshader dropped official support for them so we won't be
# able to compile these at head anymore.
ADD --chown=skia:skia https://storage.googleapis.com/skia-swiftshader/libGLESv2.so /usr/local/lib/libGLESv2.so
ADD --chown=skia:skia https://storage.googleapis.com/skia-swiftshader/libEGL.so /usr/local/lib/libEGL.so
