ARG CIPD_ROOT="/cipd"

# Keep the tag for base-cipd in sync with the tag used here for debian.
FROM debian:testing-slim AS base
RUN apt-get update && apt-get upgrade -y && apt-get install -y  \
  ca-certificates \
  && rm -rf /var/lib/apt/lists/* \
  && addgroup --gid 2000 skia \
  && adduser --uid 2000 --gid 2000 skia
USER skia:skia

# Install the CIPD client by syncing depot_tools to the revision specified in
# recipes.cfg (we're not a recipe, but it's conveniently pinned there and auto-
# rolled) and running the wrapper script. This process requires temporarily
# installing some packages that we prefer to obtain via CIPD.
FROM base AS install_cipd
USER root
RUN apt-get update && apt-get upgrade -y && apt-get install -y git curl python2-minimal
USER skia:skia
COPY ./tmp/recipes.cfg /tmp/recipes.cfg
RUN cat /tmp/recipes.cfg | \
  python2 -c "import json; import sys; print json.load(sys.stdin)['deps']['depot_tools']['revision']" > \
  /tmp/depot_tools_rev \
  && cd $(mktemp -d) \
  && git clone https://chromium.googlesource.com/chromium/tools/depot_tools.git \
  && cd depot_tools \
  && git reset --hard "$(cat /tmp/depot_tools_rev)" \
  && ./cipd --version \
  && cp ./.cipd_client /tmp/cipd

# This stage brings us back to the base image, plus the CIPD binary.
FROM base AS cipd
USER root
COPY --from=install_cipd /tmp/cipd /usr/local/bin/cipd
USER skia:skia

# Now install the desired packages.
FROM cipd AS install_pkgs
ARG CIPD_ROOT
ENV CIPD_ROOT=$CIPD_ROOT
USER root
RUN mkdir -p ${CIPD_ROOT} && chown skia:skia ${CIPD_ROOT}
USER skia
COPY ./tmp/cipd.ensure /tmp/cipd.ensure
ENV CIPD_CACHE_DIR="/tmp/.cipd_cache"
RUN cipd ensure -root=${CIPD_ROOT} -ensure-file /tmp/cipd.ensure

# The final stage brings us back to the base image with the installed CIPD packages.
FROM base AS base-cipd
ARG CIPD_ROOT
ENV CIPD_ROOT=$CIPD_ROOT
COPY --from=install_pkgs ${CIPD_ROOT} ${CIPD_ROOT}
ENV PATH="${CIPD_ROOT}:${CIPD_ROOT}/cipd_bin_packages:${CIPD_ROOT}/cipd_bin_packages/bin:${CIPD_ROOT}/cipd_bin_packages/cpython:${CIPD_ROOT}/cipd_bin_packages/cpython/bin:${CIPD_ROOT}/cipd_bin_packages/cpython3:${CIPD_ROOT}/cipd_bin_packages/cpython3/bin:${PATH}"
