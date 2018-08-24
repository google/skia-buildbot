# To make this image, copy an rpi image into this directory
# and run
# docker build -t serve-rpi-image ./
#
# then, to run the image
# docker run -d --privileged [...expose ports...] serve-rpi-image
#
# At run-time the environment variable IMAGE_PATH will be used to configure
# where the RPI image should be mounted and served from. It defaults to
# /opt/prod/root but can be changed with
#     docker run -e IMAGE_PATH='/opt/stage/root' ...

FROM erichough/nfs-server:1.1.1

COPY ./rpi.img /opt/rpi.img

ENV IMAGE_PATH='/opt/prod/root'
ENV NFS_VERSION=3

COPY ./setup_and_serve.sh /usr/local/bin

ENTRYPOINT ["/usr/local/bin/setup_and_serve.sh"]