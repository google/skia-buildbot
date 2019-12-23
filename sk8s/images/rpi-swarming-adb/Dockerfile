FROM raspbian/stretch:latest

RUN export DEBIAN_FRONTEND=noninteractive \
    && apt update \
    && apt upgrade -y \
    && apt install -y \
    adb \
    bash \
    ca-certificates \
    curl \
    libusb-1.0-0  \
    openssl \
    libssl-dev
#    tini

RUN     c_rehash
# Run c_rehash because https://bugs.debian.org/cgi-bin/bugreport.cgi?bug=923479

RUN ln -s /usr/lib/libusb-1.0.so.0.1.0 /usr/lib/libusb-1.0.so

COPY . /

# Expose default ADB port
EXPOSE 5037

# Hook up tini as the default init system for proper signal handling
#ENTRYPOINT ["/usr/bin/tini", "--"]

ENTRYPOINT ["/bin/bash", "-c", "sleep 2d"]


# Start the server by default
#CMD ["adb", "-a", "-P", "5037", "server", "nodaemon"]