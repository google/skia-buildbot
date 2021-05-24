FROM gcr.io/skia-public/base-cipd:testing-slim

USER root

# Install kubctl.
RUN apt-get update && apt-get upgrade -y && apt-get install -y curl bash software-properties-common gnupg2 \
    && mkdir -p /usr/local/bin \
    && curl https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl -o /usr/local/bin/kubectl \
    && chmod +x /usr/local/bin/kubectl

# Install docker.
RUN curl -fsSL https://download.docker.com/linux/debian/gpg | apt-key add - \
    # TODO(rmistry): bullseye does not have an entry in download.docker.com yet. When it does
    # update "buster" to be "$(lsb_release -cs)".
    && add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/debian buster stable" \
    && apt-get update \
    && apt-get install -y docker-ce docker-ce-cli

COPY . /

USER skia

ENTRYPOINT ["/usr/local/bin/docker_pushes_watcher"]
CMD ["--logtostderr"]
