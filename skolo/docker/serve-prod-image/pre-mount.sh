#!/usr/bin/env bash

# Mount the image at runtime, and then call into the base image's entry point to start serving.

mkdir -p /opt/docker-prod/root
mount /opt/rpi.img /opt/docker-prod/root -o ro,norecovery,offset=67108864,sizelimit=2367684608,noauto
/usr/local/bin/entrypoint.sh
