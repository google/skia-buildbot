FROM debian:testing-slim

RUN apt-get update && apt-get upgrade -y && apt-get install -y  \
  ca-certificates \
  && rm -rf /var/lib/apt/lists/* \
  && addgroup --gid 2000 skia \
  && adduser --uid 2000 --gid 2000 skia

USER skia:skia

COPY . /

ENV PATH="/cipd/git/bin:/cipd/python/bin:/cipd/python:${PATH}"
