# Does not used gcr.io/skia-public/base-cipd:testing-slim because we want people
# outside Skia Infra to be able to build this Docker image.
FROM debian:testing-slim

RUN apt-get update && \
    apt-get install -y git wget openssh-client curl procps unzip vim less build-essential gcc gcc-9 gcc-9-base libgcc-9-dev && \
    apt-get clean

RUN addgroup --gid 2000 skia \
  && adduser --uid 2000 --gid 2000 skia
USER skia:skia

RUN cd && \
    wget https://golang.org/dl/go1.16.2.linux-amd64.tar.gz && \
    tar -xzf go1.16.2.linux-amd64.tar.gz

ENV PATH="/home/skia/go/bin:${PATH}"

RUN cd && \
    git clone https://github.com/gohugoio/hugo.git && \
    cd hugo && \
    go install --tags extended

RUN cd && \
    wget https://nodejs.org/dist/v14.16.0/node-v14.16.0-linux-x64.tar.gz  && \
    tar -xzf node-v14.16.0-linux-x64.tar.gz

ENV PATH="/home/skia/node-v14.16.0-linux-x64/bin:${PATH}"

RUN cd && \
    git clone --recurse-submodules --depth 1 https://github.com/google/docsy-example.git docsy && \
    cd docsy && \
    npm install && \
    rm config.toml && \
    rm -rf content

COPY --chown=skia:skia . /

ENTRYPOINT ["/usr/local/bin/docsyserver"]
