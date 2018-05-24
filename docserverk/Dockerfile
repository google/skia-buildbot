FROM gcr.io/skia-public/basealpine:3.7

USER root

RUN apk update && apk add --no-cache

COPY . /

USER skia

ENTRYPOINT ["/bin/docserverk"]
