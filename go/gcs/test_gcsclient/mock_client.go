package test_gcsclient

//go:generate mockery --name GCSClient --dir ../ --output . --outpkg test_gcsclient

// NewMockClient returns a new mock GCSClient
func NewMockClient() *GCSClient {
	return &GCSClient{}
}
