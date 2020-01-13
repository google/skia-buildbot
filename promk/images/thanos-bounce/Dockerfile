FROM debian:bullseye-slim

RUN export DEBIAN_FRONTEND=noninteractive \
    && apt update \
    && apt upgrade -y \
    && apt install -y \
    curl \
    netcat \
    ucspi-tcp \
    psutils \
    bash

# Install kubctl.
RUN mkdir -p /usr/local/bin \
    && curl https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl -o /usr/local/bin/kubectl \
    && chmod +x /usr/local/bin/kubectl

RUN groupadd -g 2000 skia \
    && useradd -u 2000 -g 2000 skia \
    && mkdir -p /home/skia \
    && chown -R skia:skia /home/skia

COPY . /

USER skia

ENTRYPOINT ["/usr/local/bin/bouncer.sh"]
# nc -vv 127.0.0.1 9001 -c "kubectl exec -i $(kubectl get pod -l app=thanos-query -o jsonpath=\"{.items[0].metadata.name}\") -- tcpserver 127.0.0.1 9002 cat"
