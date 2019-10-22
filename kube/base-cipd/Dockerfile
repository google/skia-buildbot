# Keep the tag for base-cipd in sync with the tag used here for debian.
FROM debian:testing-slim

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
USER root
ENV CIPD_ROOT="/cipd"
ENV CIPD_CACHE_DIR="${CIPD_ROOT}/.cache"
RUN apt-get update && apt-get -y install git curl python-minimal \
    && mkdir -p ${CIPD_ROOT} && chown skia:skia ${CIPD_ROOT} \
    && rm -rf /var/lib/apt/lists/*
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
  && cp ./.cipd_client ${CIPD_ROOT}/cipd
ENV PATH="${CIPD_ROOT}:${PATH}"
USER root
RUN apt-get -y remove git python-minimal

# Run "cipd ensure" to obtain the packages we need.
USER skia:skia
COPY ./tmp/cipd.ensure /tmp/cipd.ensure
RUN cipd ensure -root=${CIPD_ROOT} -ensure-file /tmp/cipd.ensure
ENV PATH="${CIPD_ROOT}/cipd_bin_packages:${CIPD_ROOT}/cipd_bin_packages/bin:${PATH}"
