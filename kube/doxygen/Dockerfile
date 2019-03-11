FROM debian:testing-slim

RUN apt-get update && apt-get upgrade -y && apt-get install -y  \
  ca-certificates \
  doxygen \
  && rm -rf /var/lib/apt/lists/* 

COPY . /

