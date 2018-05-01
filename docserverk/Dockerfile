FROM gcr.io/skia-public/basealpine:3.7

USER root:root

RUN apk update && apk add git && rm -rf /var/cache/apk/*

ADD --chown=skia:skia . /

ENTRYPOINT ["/docserverk"]

USER skia:skia
