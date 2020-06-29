ARG base

# base is a shared stage which provides environment variables for all subsequent
# stages.
FROM $base AS base
ENV CGO_ENABLED 0
ENV GOFLAGS "-mod=readonly --trimpath -v"
ENV GOCACHE /gocache
ENV GOPATH /gopath
ENV PATH /gopath/bin:${PATH}
RUN apk add build-base

# build is a stage which copies in the current state of the repo and builds
# everything for all target platforms, populating both the build and module
# caches. Note that including the whole repo defeats Docker caching, since
# something is always bound to have changed. We accept this limitation because
# this build is performed on demand and the image is distributed to the machines
# who use it.
FROM base as build
COPY . /repo
WORKDIR /repo

ENV GOOS=darwin GOARCH=amd64
ENV TOOL_BIN=/gopath/bin/${GOOS}-${GOARCH}
RUN go build ./... && go test -exec=true ./... && mkdir -p ${TOOL_BIN} && \
    go build -o ${TOOL_BIN} $(go env GOROOT)/src/cmd/test2json && \
    go build -o ${TOOL_BIN} go.chromium.org/luci/client/cmd/isolate && \
    go build -o ${TOOL_BIN} go.chromium.org/luci/client/cmd/isolated

ENV GOOS=windows GOARCH=amd64
ENV TOOL_BIN=/gopath/bin/${GOOS}-${GOARCH}
RUN go build ./... && go test -exec=true ./... && mkdir -p ${TOOL_BIN} && \
    go build -o ${TOOL_BIN} $(go env GOROOT)/src/cmd/test2json && \
    go build -o ${TOOL_BIN} go.chromium.org/luci/client/cmd/isolate && \
    go build -o ${TOOL_BIN} go.chromium.org/luci/client/cmd/isolated

ENV GOOS=linux GOARCH=arm
ENV TOOL_BIN=/gopath/bin/${GOOS}-${GOARCH}
RUN go build ./... && go test -exec=true ./... && mkdir -p ${TOOL_BIN} && \
    go build -o ${TOOL_BIN} $(go env GOROOT)/src/cmd/test2json && \
    go build -o ${TOOL_BIN} go.chromium.org/luci/client/cmd/isolate && \
    go build -o ${TOOL_BIN} go.chromium.org/luci/client/cmd/isolated

# linux-amd64 is last, so that we're not cross-compiling during subsequent steps.
ENV GOOS=linux GOARCH=amd64
ENV TOOL_BIN=/gopath/bin/${GOOS}-${GOARCH}
RUN go build ./... && go test -exec=true ./... && mkdir -p ${TOOL_BIN} && \
    go build -o ${TOOL_BIN} $(go env GOROOT)/src/cmd/test2json && \
    go build -o ${TOOL_BIN} go.chromium.org/luci/client/cmd/isolate && \
    go build -o ${TOOL_BIN} go.chromium.org/luci/client/cmd/isolated

# --race on linux-amd64 only.
RUN CGO_ENABLED=1 GOFLAGS="${GOFLAGS} --race" TOOL_BIN=/gopath/bin/${GOOS}-${GOARCH}-race \
    go build ./... && go test -exec=true ./... && mkdir -p ${TOOL_BIN} && \
    go build -o ${TOOL_BIN} $(go env GOROOT)/src/cmd/test2json && \
    go build -o ${TOOL_BIN} go.chromium.org/luci/client/cmd/isolate && \
    go build -o ${TOOL_BIN} go.chromium.org/luci/client/cmd/isolated

# Install the "go-build" task driver. This is required because "go-build" itself
# builds task drivers, resulting in a chicken-and-egg problem.
RUN go install ./infra/bots/task_drivers/go_build

# This is the final stage, which just takes the environment variables from the
# "base" stage and adds the contents of the GOCACHE and GOPATH.
FROM base
COPY --from=build /gocache /gocache
COPY --from=build /gopath /gopath
COPY ./go_deps/copy_test.sh /bin/copy_test.sh
RUN chmod a+rx /bin/copy_test.sh
