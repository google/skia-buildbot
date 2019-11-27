#FROM gcr.io/skia-public/basealpine:3.8
FROM gcr.io/skia-public/base-cipd:testing-slim

USER root

# Install kubctl.
RUN apt-get update && apt-get upgrade -y && apt-get install -y curl bash software-properties-common gnupg2 sudo uidmap \
    && mkdir -p /usr/local/bin \
    && curl https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl -o /usr/local/bin/kubectl \
    && chmod +x /usr/local/bin/kubectl

# Install docker.
RUN curl -fsSL https://download.docker.com/linux/debian/gpg | apt-key add - \
    && add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/debian stretch stable" \
    && apt-get update \
    && apt-get install -y docker-ce docker-ce-cli \
    && service docker start \
    # && groupadd docker \
    && usermod -aG docker skia
    # && chown root:docker /var/run/docker.sock
    # && dockerd

COPY . /

USER skia

ENV SKIP_IPTABLES 1
RUN curl -fsSL https://get.docker.com/rootless | sh


ENTRYPOINT ["/usr/local/bin/docker_pushes_watcher"]
CMD ["--logtostderr"]
