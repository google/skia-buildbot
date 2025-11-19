# TODO(borenet): Obtain this from MODULE.bazel!
FROM gcr.io/skia-public/base-cipd@sha256:d775388db67ebb4d9cd36a759aaee785a3cf4a3999120f57ed9135e1445eb9d5

USER root
RUN apt-get update \
    && apt-get install -y \
        wget \
        openssh-client \
        procps \
        unzip \
        vim \
        less \
        build-essential \
        gcc \
        gcc-12 \
        gcc-12-base \
        libgcc-12-dev \
    && apt-get clean

USER skia

# Install golang.
RUN cd /home/skia \
    && wget https://golang.org/dl/go1.16.2.linux-amd64.tar.gz \
    && tar -xzf go1.16.2.linux-amd64.tar.gz

# Install hugo at a pinned version we know works.
RUN cd /home/skia \
    && git clone https://github.com/gohugoio/hugo.git --depth=1 --revision=803f572e66c5e22213ddcc994c41b3e80e9c1f35 \
    && cd hugo \
    && ~/go/bin/go install --tags extended

# Install node.
RUN cd /home/skia \
    && wget https://nodejs.org/dist/v14.16.0/node-v14.16.0-linux-x64.tar.gz \
    && tar -xzf node-v14.16.0-linux-x64.tar.gz

# Install docsy at a pinned version we know works.
RUN cd /home/skia \
    && git clone --recurse-submodules --depth=1 --revision=70e301f7861122ab129d2c46ee5ed625e92c04d0 https://github.com/google/docsy-example.git docsy \
    && cd docsy \
    && PATH="/home/skia/node-v14.16.0-linux-x64/bin:${PATH}" /home/skia/node-v14.16.0-linux-x64/bin/npm install \
    && rm config.toml \
    && rm -rf content
