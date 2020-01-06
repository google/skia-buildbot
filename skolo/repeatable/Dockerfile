FROM arm32v7/python:2.7-alpine3.9

RUN apk update && apk add --no-cache \
    ca-certificates \
    curl \
    bash \
    git

RUN git clone https://chromium.googlesource.com/chromium/tools/depot_tools.git /root/depot_tools

COPY /usr/bin/qemu-arm-static /usr/bin/qemu-arm-static

CMD ["/bin/bash"]
