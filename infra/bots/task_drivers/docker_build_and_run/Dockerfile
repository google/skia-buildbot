# Dockerfile which builds an image containing the docker_build task driver.
#
# Expects the docker_build executable to be present in the context.
ARG BASE_IMAGE="alpine:3.8"

FROM $BASE_IMAGE
COPY ./docker_build_and_run ./docker_build_and_run
