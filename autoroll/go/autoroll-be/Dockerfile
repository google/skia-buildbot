FROM gcr.io/skia-public/base-cipd:testing-slim

USER root

RUN apt-get update && apt-get install -y wget openssh-client curl procps unzip vim less build-essential g++ g++-10 gcc gcc-10 gcc-10-base cpp cpp-10 libgcc-10-dev libstdc++-10-dev cmake

USER skia

COPY . /
