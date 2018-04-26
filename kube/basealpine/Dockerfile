FROM alpine

RUN addgroup -g 2000 -S skia && \
    adduser -u 2000 -S skia -G skia

USER skia:skia

ADD . /
