FROM debian:testing-slim

RUN apt-get update && apt-get upgrade -y && apt-get install -y  \
  ca-certificates \
  && rm -rf /var/lib/apt/lists/* \
  && addgroup --gid 2000 skia \
  && adduser --uid 2000 --gid 2000 skia

USER skia:skia

COPY . /

USER root

RUN chown -R skia:skia /cipd

USER skia:skia

ENV CIPD_CACHE_DIR="/cipd/.cache"
ENV CIPD_ROOT="/cipd"
ENV PATH="/cipd:${PATH}"
