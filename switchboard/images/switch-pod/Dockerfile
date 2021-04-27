FROM alpine:3.13

USER root

RUN apk update && \
    apk add --no-cache procps netcat-openbsd

COPY . /

ENTRYPOINT ["/usr/local/bin/sleeper.sh"]