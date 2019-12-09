FROM debian:stable

USER root

# Create skia
RUN addgroup --gid 2000 skia \
  && adduser --uid 2000 --gid 2000 skia

# Install kubctl.
RUN apt-get update && apt-get upgrade -y && apt-get install -y curl bash software-properties-common gnupg2 \
    && mkdir -p /usr/local/bin \
    && curl https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl -o /usr/local/bin/kubectl \
    && chmod +x /usr/local/bin/kubectl

# Install docker.
RUN curl -fsSL https://download.docker.com/linux/debian/gpg | apt-key add - \
    && add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/debian $(lsb_release -cs) stable" \
    && apt-get update \
    && apt-get install -y docker-ce docker-ce-cli \
    && service docker start

COPY . /

USER skia

ENTRYPOINT ["/usr/local/bin/docker_pushes_watcher"]
CMD ["--logtostderr"]
