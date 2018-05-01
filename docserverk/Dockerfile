FROM debian:testing-slim
RUN apt-get update && apt-get install -y \
  git

ADD . /

ENTRYPOINT ["/usr/local/bin/docserverk"]

