package releaseinfra

import (
	"go.skia.org/infra/mcp/common"
	rivh "go.skia.org/infra/mcp/services/releaseinfra/versionhistory"
)

type ReleaseInfraService struct {
	// The Version History API client.
	vhClient rivh.VersionHistoryClient
}

// Initialize the service with the provided arguments.
func (s *ReleaseInfraService) Init(serviceArgs string) error {
	s.vhClient = *rivh.NewVersionHistoryClient()

	return nil
}

// GetTools returns the supported tools by the service.
func (s ReleaseInfraService) GetTools() []common.Tool {
	return rivh.GetTools(&s.vhClient)
}

func (s *ReleaseInfraService) GetResources() []common.Resource {
	return []common.Resource{}
}

func (s *ReleaseInfraService) Shutdown() error {
	return nil
}
