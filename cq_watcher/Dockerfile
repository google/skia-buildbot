FROM node:8-alpine

RUN addgroup -g 2000 -S skia && \
    adduser -u 2000 -S skia -G skia && \
    apk update && \
    apk add --no-cache git make ca-certificates

COPY . /

USER skia

ENTRYPOINT ["/cq_watcher"]
CMD ["--logtostderr", "--prom_port=:20000"]
