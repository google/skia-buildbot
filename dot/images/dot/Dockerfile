FROM gcr.io/skia-public/basealpine:3.8

USER root

RUN apk update && \
    apk add --no-cache ca-certificates graphviz

COPY . /

USER skia

ENTRYPOINT ["/usr/local/bin/dotserver"]