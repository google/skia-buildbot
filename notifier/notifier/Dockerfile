FROM gcr.io/skia-public/basealpine:3.8

USER root

RUN apk add --no-cache tzdata

COPY . /

USER skia

ENTRYPOINT ["/usr/local/bin/notifier"]
