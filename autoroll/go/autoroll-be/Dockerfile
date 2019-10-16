FROM gcr.io/skia-public/base-cipd:testing-slim

USER root

RUN apt-get update && apt-get install -y build-essential wget openssh-client curl procps unzip vim less

USER skia

COPY . /
