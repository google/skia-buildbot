# Dockerfile extending the generic Go image with application files for a
# single application.
FROM golang:1.6

ENV mainpkg "github.com/luci/luci-go/logdog/server/cmd/logdog_archivist"

# Copy the local package files to the container's workspace.
ADD _gopath/src/ /go/src
ADD . "/go/src/${mainpkg}"
COPY run.sh /opt/logdog/archivist/run.sh

# Build the command inside the container.
# (You may fetch or manage dependencies here,
# either manually or with a tool like "godep".)
RUN go install "${mainpkg}"

# Run the output command by default when the container starts.
ENTRYPOINT ["/bin/bash", "/opt/logdog/archivist/run.sh", "/go/bin/logdog_archivist"]
