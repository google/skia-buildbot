FROM debian:testing-slim
RUN apt-get update && apt-get install -y \
  git

ADD . /

EXPOSE 8000 20000

CMD ["/usr/local/bin/docserver", "--logtostderr", "--resources_dir=/usr/local/share/docserver/", "--work_dir=/mnt/pd0"]
