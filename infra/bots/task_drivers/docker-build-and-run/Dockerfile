# Dockerfile which builds an image containing the docker_build task driver.
#
# Expects the docker-build-and-run executable to be present in the context.
ARG BASE_IMAGE="alpine:3.8"

FROM $BASE_IMAGE
COPY ./docker-build-and-run /usr/bin/docker-build-and-run
ENTRYPOINT ["/usr/bin/docker-build-and-run"]
