FROM node:8-alpine

RUN addgroup -g 2000 -S skia && \
    adduser -u 2000 -S skia -G skia

COPY . /

USER skia

ENTRYPOINT ["/jsdoc"]
CMD ["--port=:8000", "--prom_port=:20000", "--local"]
