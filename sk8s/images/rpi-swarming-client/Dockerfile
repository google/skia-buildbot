FROM raspbian/stretch:latest AS libimobiledevice-build

RUN export DEBIAN_FRONTEND=noninteractive \
    && apt update \
    && apt upgrade -y \
    && apt install -y \
    build-essential \
    autoconf \
    checkinstall \
    libtool \
    libzip-dev \
    fuse \
    libusb-1.0-0-dev \
    libfuse-dev \
    libxml2-dev \
    libssl-dev

WORKDIR /build
COPY libimobiledevice-build.sh .
RUN libimobiledevice-build.sh

RUN export DEBIAN_FRONTEND=noninteractive \
    && apt update \
    && apt upgrade -y \
    && apt install -y \
    python2.7 \
    python-certifi \
    ca-certificates \
    curl \
    libusb-1.0-0 \
    bash \
    openssl \
    libssl-dev \
    adb \
    libzip2

RUN     c_rehash
# Run c_rehash because https://bugs.debian.org/cgi-bin/bugreport.cgi?bug=923479

RUN ln -s /usr/lib/libusb-1.0.so.0.1.0 /usr/lib/libusb-1.0.so
RUN ln -s /usr/bin/adb /usr/bin/adb.1.0.35

COPY . /

# Install libimobiledevice
COPY --from=libimobiledevice-build /build/out/sbin   /sbin
COPY --from=libimobiledevice-build /build/out/bin    /bin
COPY --from=libimobiledevice-build /build/out/lib    /lib
COPY --from=libimobiledevice-build /build/out/share  /share
COPY --from=libimobiledevice-build /build/out/udev-rules/39-usbmuxd.rules \
    /etc/udev/rules.d/39-usbmuxd.rules
COPY --from=libimobiledevice-build /build/out/systemd/usbmuxd.service \
    /etc/systemd/system/usbmuxd.service
RUN /libimobiledevice-postinst.sh

# Swarming requires a .boto file, even if it's empty.
RUN touch /root/.boto

# Swarming can only detect running inside a Docker container, it can't detect
# containerd, so let's fake it out.
# https://cs.chromium.org/chromium/infra/luci/appengine/swarming/swarming_bot/api/platforms/linux.py?type=cs&q=get_inside_docker&sq=package:chromium&g=0
RUN touch /.dockerenv

CMD ["/start_swarming"]