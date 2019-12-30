FROM gcr.io/skia-public/base-cipd:testing-slim

USER root

RUN apt-get update && apt-get install -y file ca-certificates procps wget

# Create required dirs.
RUN mkdir --parents /b/storage/ && chown skia:skia /b/storage

USER skia

COPY . /

ENTRYPOINT ["/usr/local/bin/ctfe"]
