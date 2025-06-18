package releaseinfra

import (
	"context"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/mcp/common"
	sc "go.skia.org/infra/mcp/services/common"
	ribb "go.skia.org/infra/mcp/services/releaseinfra/buildbucket"
	// The main Buildbucket v2 client library.
	// The PRPC client is used to communicate with LUCI services.
)

type ReleaseInfraService struct {
	// The Buildbucket client used to interact with the Buildbucket service.
	bbClient ribb.BuildbucketClient
}

// Initialize the service with the provided arguments.
func (s *ReleaseInfraService) Init(serviceArgs string) error {
	ctx := context.Background()
	httpClient, err := sc.DefaultHttpClient(ctx)
	if err != nil {
		return skerr.Wrapf(err, "Failed to create http client")
	}
	s.bbClient = ribb.NewBuildbucketClient(httpClient)

	return nil
}

// GetTools returns the supported tools by the service.
func (s ReleaseInfraService) GetTools() []common.Tool {
	return ribb.GetTools(&s.bbClient)
}

func (s *ReleaseInfraService) GetResources() []common.Resource {
	return []common.Resource{}
}

func (s *ReleaseInfraService) Shutdown() error {
	return nil
}
