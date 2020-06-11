ARG base=golang:1.14
FROM $base

# Setup.
ENV GOFLAGS -mod=readonly
WORKDIR /repo
COPY . /repo

# Download and build dependencies for all platforms.
RUN date +%s > /root/started.txt && ./install-all.sh

# Cleanup.
WORKDIR /
RUN rm -rf /repo