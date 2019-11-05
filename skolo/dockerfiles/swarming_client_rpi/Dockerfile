FROM arm32v7/debian:bullseye-slim

COPY /usr/bin/qemu-arm-static /usr/bin

RUN useradd -ou 0 -g 0 chromebot && \
    export DEBIAN_FRONTEND=noninteractive && \
    apt-get update && \
    apt-get -y upgrade && \
    apt-get install -y python2.7 curl ca-certificates && \
    c_rehash
# Run c_rehash because https://bugs.debian.org/cgi-bin/bugreport.cgi?bug=923479

COPY . /

RUN touch /root/.boto

USER chrome-bot

CMD ["/start_swarming"]