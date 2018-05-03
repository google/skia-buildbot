FROM gcr.io/skia-public/basealpine:3.7

USER root

RUN apk update && apk add git && rm -rf /var/cache/apk/*

ADD . /

USER skia

ENTRYPOINT ["/bin/docserverk"]
