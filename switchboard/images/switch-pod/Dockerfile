FROM debian:testing-slim

RUN apt-get update && \
    apt-get install -y openssh-client curl procps netcat && \
    apt-get clean

USER root

COPY . /

ENTRYPOINT ["/usr/local/bin/switch-pod-monitor", "--config=/etc/switch-pod-monitor/config.json"]