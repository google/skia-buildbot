FROM gcr.io/skia-public/base-cipd:testing-slim

USER root

RUN apt-get update && apt-get install -y wget openssh-client curl procps unzip vim less build-essential g++ g++-9 gcc gcc-9 gcc-9-base cpp cpp-9 libgcc-9-dev libstdc++-9-dev

USER skia

COPY . /
