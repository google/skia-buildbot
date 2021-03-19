FROM gcr.io/skia-public/base-cipd:testing-slim

USER root

RUN apt-get update && apt-get install -y wget openssh-client curl procps unzip vim less build-essential g++ g++-9 gcc gcc-9 gcc-9-base cpp cpp-9 libgcc-9-dev libstdc++-9-dev


RUN wget https://golang.org/dl/go1.16.2.linux-amd64.tar.gz && \
    tar -xzf go1.16.2.linux-amd64.tar.gz

ENV PATH="/go/bin:${PATH}"

RUN git clone https://github.com/gohugoio/hugo.git && \
    cd hugo && \
    go install --tags extended

RUN wget https://nodejs.org/dist/v14.16.0/node-v14.16.0-linux-x64.tar.gz  && \
    tar -xzf node-v14.16.0-linux-x64.tar.gz

ENV PATH="/node-v14.16.0-linux-x64/bin:${PATH}"

COPY . /

RUN cd / && \
    git clone --recurse-submodules --depth 1 https://github.com/google/docsy-example.git docsy && \
    cd docsy && \
    npm install && \
    rm config.toml && \
    rm -rf content

ENTRYPOINT ["/serve.sh"]
