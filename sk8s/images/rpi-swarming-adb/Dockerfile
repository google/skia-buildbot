FROM arm32v7/debian:bullseye-slim

COPY /usr/bin/qemu-arm-static /usr/bin

RUN export DEBIAN_FRONTEND=noninteractive && \
    apt-get update && \
    apt-get -y upgrade && \
    apt-get install -y tini ca-certificates adb && \
    c_rehash
# Run c_rehash because https://bugs.debian.org/cgi-bin/bugreport.cgi?bug=923479

COPY . /

EXPOSE 5037

ENTRYPOINT ["/usr/bin/tini", "--"]
#CMD ["/usr/bin/adb", "-a", "-P", "5037", "server", "nodaemon"]
CMD ["/bin/bash", "-c", "sleep 2d"]