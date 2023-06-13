# Keep the tag for basealpine in sync with the tag used here for alpine.
FROM alpine:3.9

RUN apk update && apk add --no-cache ca-certificates adduser && \
    addgroup -g 2000 -S skia && \
    adduser -u 2000 -S skia -G skia

USER skia:skia
