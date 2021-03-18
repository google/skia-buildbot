FROM klakegg/hugo:ext-debian as hugo

FROM gcr.io/skia-public/base-cipd:testing-slim

USER root

RUN apt-get update && apt-get install -y wget

RUN wget https://nodejs.org/dist/v14.16.0/node-v14.16.0-linux-x64.tar.gz  && \
    tar -xzf node-v14.16.0-linux-x64.tar.gz

ENV PATH="/node-v14.16.0-linux-x64/bin:${PATH}"

RUN npm install -D --save autoprefixer
RUN npm install -D --save postcss-cli
RUN npm install -D --save postcss@^8.1.0

COPY . /

COPY --from=hugo / /

RUN cd / && \
    git clone --recurse-submodules --depth 1 https://github.com/google/docsy-example.git && \
    rm docsy-example/config.toml

ENTRYPOINT ["/serve.sh"]
