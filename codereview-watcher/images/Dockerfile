FROM gcr.io/skia-public/basealpine:3.8

USER root

RUN apk update && \
    apk add --no-cache bash procps

COPY . /

USER skia
