FROM gcr.io/skia-public/basedebian:testing-slim

USER root

RUN apt-get update && apt-get install -y procps gpg curl vim jq kubectl

USER skia

COPY . /

ENTRYPOINT ["/usr/local/bin/alerts_checker"]
