# To make this image, copy an rpi image into this directory
# and run
# docker build -t serve-prod-with-image ./
#
# then, to run the image
# docker run -d --privileged [...expose ports...] serve-prod-with-image

FROM erichough/nfs-server:1.1.1

COPY ./rpi.img /opt/rpi.img

# can be over-ridden with docker run -e IMAGE_PATH='/opt/stage/root ...'
ENV IMAGE_PATH='/opt/prod/root'

COPY ./setup_and_serve.sh /usr/local/bin
ENTRYPOINT ["/usr/local/bin/setup_and_serve.sh"]