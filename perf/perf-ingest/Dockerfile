FROM gcr.io/skia-public/basealpine:3.7

USER root

RUN apk --no-cache add git

COPY . /

USER skia

ENTRYPOINT ["/usr/local/bin/perf-ingest"]
CMD ["--logtostderr"]
