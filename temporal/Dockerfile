# syntax=docker/dockerfile:1

# Build the application from source
FROM google-go.pkg.dev/golang:1.21.3@sha256:80ccdc8f8ac8d819cdbc15a33334125e0288c09ac030307dcd893d2b5c6179ae AS build-stage

WORKDIR /app

RUN mkdir -p /tmp/temporal

RUN CGO_ENABLED=0 GOOS=linux GOBIN=/tmp/temporal go install go.temporal.io/server/cmd/server@v1.22.2

# Deploy the application binary into a lean image.
# Tagged as 3.9 Jan 31, 2022.
FROM gcr.io/skia-public/basealpine@sha256:35a26930eb37b90cb0bdf69050e363bd749b56656963b78c8c4b4758a5aea8fa AS build-release-stage

WORKDIR /

COPY --from=build-stage /tmp/temporal/server /temporal

ENTRYPOINT ["/temporal"]