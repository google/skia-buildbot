FROM debian:bullseye-slim

RUN export DEBIAN_FRONTEND=noninteractive \
    && apt update \
    && apt upgrade -y \
    && apt install -y \
    curl \
    netcat-traditional \
    ucspi-tcp \
    bash

COPY . /

ENTRYPOINT ["/usr/local/bin/thanos"]