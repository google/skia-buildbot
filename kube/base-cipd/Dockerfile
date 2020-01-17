ARG CIPD_ROOT="/cipd"

# Install the CIPD client by syncing depot_tools to the revision specified in
# recipes.cfg (we're not a recipe, but it's conveniently pinned there and auto-
# rolled) and running the wrapper script. This process requires temporarily
# installing some packages that we prefer to obtain via CIPD.
FROM gcr.io/skia-public/basedebian:testing-slim AS install_cipd
USER root
RUN apt-get update && apt-get upgrade -y && apt-get install -y git curl python-minimal
USER skia:skia
COPY ./tmp/recipes.cfg /tmp/recipes.cfg
RUN cat /tmp/recipes.cfg | \
    python -c "import json; import sys; print json.load(sys.stdin)['deps']['depot_tools']['revision']" > \
    /tmp/depot_tools_rev \
  && cd $(mktemp -d) \
  && git clone https://chromium.googlesource.com/chromium/tools/depot_tools.git \
  && cd depot_tools \
  && git reset --hard "$(cat /tmp/depot_tools_rev)" \
  && ./cipd --version \
  && cp ./.cipd_client /tmp/cipd

# This stage brings us back to the base image, plus the CIPD binary.
FROM gcr.io/skia-public/basedebian:testing-slim AS cipd
USER root
COPY --from=install_cipd /tmp/cipd /usr/local/bin/cipd
USER skia:skia
ARG CIPD_ROOT
ENV CIPD_ROOT=$CIPD_ROOT
ENV CIPD_CACHE_DIR="/tmp/.cipd_cache"
ENV PATH="${CIPD_ROOT}:${CIPD_ROOT}/cipd_bin_packages:${CIPD_ROOT}/cipd_bin_packages/bin:${PATH}"
USER root
RUN mkdir -p ${CIPD_ROOT} && chown skia:skia ${CIPD_ROOT}
USER skia

# Now install the desired packages.
#FROM cipd AS install_pkgs
#COPY ./tmp/cipd.ensure /tmp/cipd.ensure
#RUN cipd ensure -root=${CIPD_ROOT} -ensure-file /tmp/cipd.ensure

# The final stage brings us back to the base image with the installed CIPD packages.
#FROM gcr.io/skia-public/basedebian:testing-slim AS base-cipd
#ARG CIPD_ROOT
#ENV CIPD_ROOT=$CIPD_ROOT
#COPY --from=install_pkgs ${CIPD_ROOT} ${CIPD_ROOT}
#ENV PATH="${CIPD_ROOT}:${CIPD_ROOT}/cipd_bin_packages:${CIPD_ROOT}/cipd_bin_packages/bin:${PATH}"
