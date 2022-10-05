package mocks

//go:generate bazelisk run --config=mayberemote //:mockery   -- --name GCSUploader  --srcpkg=go.skia.org/infra/gold-client/go/gcsuploader --output ${PWD}
//go:generate bazelisk run --config=mayberemote //:mockery   -- --name HTTPClient  --srcpkg=go.skia.org/infra/gold-client/go/httpclient --output ${PWD}
//go:generate bazelisk run --config=mayberemote //:mockery   -- --name ImageDownloader  --srcpkg=go.skia.org/infra/gold-client/go/imagedownloader --output ${PWD}
//go:generate bazelisk run --config=mayberemote //:mockery   -- --name Matcher  --srcpkg=go.skia.org/infra/gold-client/go/imgmatching --output ${PWD}
