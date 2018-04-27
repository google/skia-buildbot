# Keep the tag for basealpine in sync with the tag used here for alpine.
FROM alpine:3.7

RUN addgroup -g 2000 -S skia && \
    adduser -u 2000 -S skia -G skia && \
    apk update && apk add ca-certificates && rm -rf /var/cache/apk/*

USER skia:skia
