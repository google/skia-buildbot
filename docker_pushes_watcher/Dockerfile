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
    # TODO(rmistry): Docker client version 19.03.09 has this bug: https://github.com/docker/cli/issues/2533
    # The bug has been fixed in v19.03.10 but that version has not been released yet. Remove
    # the specific versions below after the version is released.
    && apt-get install -y docker-ce=5:19.03.8~3-0~debian-buster docker-ce-cli=5:19.03.8~3-0~debian-buster \
    && service docker start

COPY . /

USER skia

ENTRYPOINT ["/usr/local/bin/docker_pushes_watcher"]
CMD ["--logtostderr"]
