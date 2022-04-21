package test_gcsclient

//go:generate bazelisk run //:mockery   -- --name GCSClient  --srcpkg=go.skia.org/infra/go/gcs --output ${PWD} --outpkg test_gcsclient

// NewMockClient returns a new mock GCSClient
func NewMockClient() *GCSClient {
	return &GCSClient{}
}
