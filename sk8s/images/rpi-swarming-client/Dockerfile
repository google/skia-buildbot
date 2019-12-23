FROM arm32v7/python:2.7-alpine3.9

RUN addgroup -g 2000 -S chrome-bot && \
    adduser -u 2000 -S chrome-bot -G chrome-bot -s /bin/sh && \
    apk update && apk add --no-cache \
    --repository http://dl-cdn.alpinelinux.org/alpine/edge/testing \
    --repository http://dl-cdn.alpinelinux.org/alpine/edge/main \
    ca-certificates \
    curl \
    libusb \
    bash \
    openssl-dev \
    android-tools

RUN ln `which python2.7` /usr/bin/python2.7
RUN ln -s /usr/lib/libusb-1.0.so.0.1.0 /usr/lib/libusb-1.0.so

COPY . /

RUN touch /root/.boto

# Swarming can only detect running inside a Docker container, it can't detect
# containerd, so let's fake it out.
# https://cs.chromium.org/chromium/infra/luci/appengine/swarming/swarming_bot/api/platforms/linux.py?type=cs&q=get_inside_docker&sq=package:chromium&g=0
RUN touch /.dockerenv

CMD ["/start_swarming"]