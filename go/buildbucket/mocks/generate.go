package mocks

//go:generate bazelisk run --config=mayberemote //:mockery   -- --name BuildBucketInterface  --srcpkg=go.skia.org/infra/go/buildbucket --output ${PWD}
