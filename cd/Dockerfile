ARG CIPD_ROOT="/cipd"

FROM debian:bullseye as base
RUN apt-get update && apt-get upgrade -y && apt-get install -y  \
    apt-transport-https \
    ca-certificates \
    clang-11 \
    curl \
    gnupg2 \
    make \
    patch \
    software-properties-common \
    && curl -fsSL https://download.docker.com/linux/debian/gpg | apt-key add - \
    && add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/debian bullseye stable" \
    && apt-get update \
    && apt-get install -y docker-ce \
    && rm -rf /var/lib/apt/lists/* \
    && apt-get remove -y git \
    && ln -s /usr/bin/python3 /usr/bin/python
ENV CC=clang-11

# Install the CIPD client by syncing depot_tools to the revision specified in
# recipes.cfg (we're not a recipe, but it's conveniently pinned there and auto-
# rolled) and running the wrapper script. This process requires temporarily
# installing some packages that we prefer to obtain via CIPD.
FROM base AS install_cipd
RUN apt-get update && apt-get install -y git curl python3-minimal
COPY ./recipes.cfg /tmp/recipes.cfg
RUN cat /tmp/recipes.cfg | \
    python3 -c "import json; import sys; print(json.load(sys.stdin)['deps']['depot_tools']['revision'])" > /tmp/depot_tools_rev \
    && cd $(mktemp -d) \
    && git clone https://chromium.googlesource.com/chromium/tools/depot_tools.git \
    && cd depot_tools \
    && git reset --hard "$(cat /tmp/depot_tools_rev)" \
    && ./cipd --version \
    && cp ./.cipd_client /tmp/cipd

# This stage brings us back to the base image, plus the CIPD binary.
FROM base AS cipd
COPY --from=install_cipd /tmp/cipd /usr/local/bin/cipd

# Now install the desired packages.
FROM cipd AS install_pkgs
ARG CIPD_ROOT
ENV CIPD_ROOT=$CIPD_ROOT
RUN mkdir -p ${CIPD_ROOT}
ENV CIPD_CACHE_DIR="/tmp/.cipd_cache"
RUN cipd init ${CIPD_ROOT} -force \
    && cipd install skia/bots/bazelisk version:0 -root /cipd \
    && cipd install infra/3pp/tools/git/linux-amd64 version:2.36.0-rc2.chromium.8 -root /cipd \
    && cipd install skia/bots/go version:16 -root /cipd

# The final stage brings us back to the base image with the installed CIPD packages.
FROM base AS final
ARG CIPD_ROOT
ENV CIPD_ROOT=$CIPD_ROOT
COPY --from=install_pkgs ${CIPD_ROOT} ${CIPD_ROOT}
ENV GOPATH=/go
RUN mkdir -p ${GOPATH}
ENV PATH="${CIPD_ROOT}/bin:${CIPD_ROOT}/go/bin:${CIPD_ROOT}:${GOPATH}/bin:${PATH}"
