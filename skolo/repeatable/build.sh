#!/bin/bash

INSTALL="install -D --verbose --backup=none"
${INSTALL} --mode=755 -T /usr/bin/qemu-arm-static   ./usr/bin/qemu-arm-static
docker build -t repeatable .
