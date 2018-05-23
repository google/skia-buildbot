FROM debian:testing-slim

ADD https://storage.googleapis.com/swiftshader-binaries/OpenGL_ES/Latest/Linux/libGLESv2.so /usr/local/lib/libGLESv2.so
ADD https://storage.googleapis.com/swiftshader-binaries/OpenGL_ES/Latest/Linux/libEGL.so /usr/local/lib/libEGL.so

RUN addgroup -g 2000 -S skia && \
    adduser -u 2000 -S skia -G skia && \
    apt-get update && apt-get upgrade -y && apt-get install -y  \
    libfontconfig1 \
    libglu1-mesa \
    xvfb \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=gcr.io/skia-public/skiaserve:latest /bin/skiaserve /skiaserve

COPY . /

USER skia

ENTRYPOINT ["/debugger"]
CMD ["--logtostderr", "--resources_dir=/usr/local/share/debugger"]
