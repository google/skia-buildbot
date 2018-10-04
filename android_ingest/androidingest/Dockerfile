FROM gcr.io/skia-public/basealpine:3.7

USER root

RUN apk update && \
    apk add --no-cache git ca-certificates

COPY . /

USER skia

ENTRYPOINT ["/usr/local/bin/androidingest"]
CMD ["--logtostderr"]
