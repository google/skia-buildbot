FROM gcr.io/skia-public/basealpine:3.7

USER root

RUN apk update && \
    apk add --no-cache git ca-certificates tzdata

COPY . /

USER skia

ENTRYPOINT ["/usr/local/bin/skiaperf"]
CMD ["--logtostderr"]
