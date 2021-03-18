FROM gcr.io/skia-public/base-cipd:testing-slim

USER root

RUN apt-get update && apt-get install -y wget openssh-client curl less build-essential gcc gcc-9 gcc-9-base libgcc-9-dev

COPY . /

RUN cd / && \
    wget https://golang.org/dl/go1.16.2.linux-amd64.tar.gz && \
    tar -xzf go1.16.2.linux-amd64.tar.gz && \
    git clone https://github.com/gohugoio/hugo.git && \
    cd hugo; ../go/bin/go install --tags extended && \
    pwd

RUN cd / && \
    git clone --recurse-submodules --depth 1 https://github.com/google/docsy-example.git && \
    rm docsy-example/config.toml

ENTRYPOINT ["/serve.sh"]
