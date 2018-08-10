FROM debian:9.5

USER root

RUN apt-get update && apt-get install -y git file ca-certificates

#Add user so we don't have to run as root (prevents us from over-writing files in /SRC)
RUN groupadd -g 2000 skia \
    && useradd -u 2000 -g 2000 skia \
    && mkdir -p /home/skia \
    && chown -R skia:skia /home/skia

USER skia

COPY . /

RUN mkdir -p /tmp/workdir && \
    cd /tmp/workdir && \
    git clone https://github.com/luci/client-py.git

ENV PATH /tmp/workdir/client-py:/tmp/workdir/:$PATH

ENTRYPOINT ["/leasing"]
# TODO(rmistry): Will have to add more here.
CMD ["--logtostderr", "--prom_port=:20000", "--resources_dir=/usr/local/share/leasing/", "--workdir=/tmp/workdir"]

