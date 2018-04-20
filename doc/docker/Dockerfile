FROM debian:testing-slim
RUN apt-get update && apt-get install -y \
  git

ADD . /

EXPOSE 8000 20000

ENTRYPOINT ["/usr/local/bin/docserver"]

