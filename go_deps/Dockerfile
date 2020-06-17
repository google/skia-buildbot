ARG base

# base is a shared stage which provides environment variables for all subsequent
# stages.
FROM $base AS base
ENV CGO_ENABLED 0
ENV GOFLAGS -mod=readonly --trimpath -v
ENV GOCACHE /gocache
ENV GOPATH /gopath
ENV PATH ${GOPATH}/bin:${PATH}

# build is a stage which copies in the current state of the repo and builds
# everything for all target platforms, populating both the build and module
# caches. Note that including the whole repo defeats Docker caching, since
# something is always bound to have changed. We accept this limitation because
# this build is performed on demand and the image is distributed to the machines
# who use it.
FROM base as build
COPY . /repo
WORKDIR /repo
RUN GOOS=linux   GOARCH=amd64 go build ./... && go test -exec=true ./...
RUN GOOS=linux   GOARCH=arm   go build ./... && go test -exec=true ./...
RUN GOOS=darwin  GOARCH=amd64 go build ./... && go test -exec=true ./...
RUN GOOS=windows GOARCH=amd64 go build ./... && go test -exec=true ./...

# Install the "go-build" task driver. This is required because "go-build" itself
# builds task drivers, resulting in a chicken-and-egg problem.
RUN go install ./infra/bots/task_drivers/go_build

# This is the final stage, which just takes the environment variables from the
# "base" stage and adds the contents of the GOCACHE and GOPATH.
FROM base
COPY --from=build /gocache /gocache
COPY --from=build /gopath /gopath
