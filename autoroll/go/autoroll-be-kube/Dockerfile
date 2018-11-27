FROM gcr.io/skia-public/basedebian:testing-slim

USER root

RUN apt-get update && apt-get install -y build-essential wget openssh-client curl procps

USER skia

COPY . /

ENTRYPOINT ["/usr/local/bin/autoroll-be"]
