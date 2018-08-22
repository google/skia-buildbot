# To make this image, copy an rpi image into this directory
# and run
# docker build -t serve-prod-with-image ./
#
# then, to run the image
# docker run -d --privileged [...expose ports...] serve-prod-with-image

FROM erichough/nfs-server:1.1.1

COPY ./docker-prod.img /opt/rpi.img

COPY ./exports /etc/exports

COPY ./pre-mount.sh /usr/local/bin
ENTRYPOINT ["/usr/local/bin/pre-mount.sh"]