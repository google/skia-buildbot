FROM arm32v7/debian:bullseye-slim

COPY /usr/bin/qemu-arm-static /usr/bin

RUN export DEBIAN_FRONTEND=noninteractive && \
    apt-get update && \
    apt-get -y upgrade && \
    apt-get install -y python2.7 curl ca-certificates && \
    c_rehash

COPY . /

RUN touch /root/.boto

CMD ["/start_swarming"]