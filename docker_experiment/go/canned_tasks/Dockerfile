# Dockerfile which builds an image containing the canned steps.
#
# Expects the steps to be present in ./bin of the context.

ARG BASE=docker

FROM $BASE
COPY ./bin /bin
WORKDIR /work
