package mocks

//go:generate bazelisk run //:mockery   -- --name GCSUploader  --srcpkg=go.skia.org/infra/gold-client/go/gcsuploader --output ${PWD}
//go:generate bazelisk run //:mockery   -- --name HTTPClient  --srcpkg=go.skia.org/infra/gold-client/go/httpclient --output ${PWD}
//go:generate bazelisk run //:mockery   -- --name ImageDownloader  --srcpkg=go.skia.org/infra/gold-client/go/imagedownloader --output ${PWD}
//go:generate bazelisk run //:mockery   -- --name Matcher  --srcpkg=go.skia.org/infra/gold-client/go/imgmatching --output ${PWD}
