package luciconfig

import (
	"context"

	configApi "go.chromium.org/luci/common/api/luci_config/config/v1"
	"go.skia.org/infra/go/skerr"
)

const (
	API_BASE_PATH = "https://luci-config.appspot.com/_ah/api/config/v1/"
)

// Interface for LUCI Config wrapper.
type ApiClient interface {
	// Given a LUCI Config path, retrieve all matching configs to that path.
	GetProjectConfigs(path string) ([]*configApi.LuciConfigGetConfigMultiResponseMessageConfigEntry, error)
}

type apiClient struct {
	s *configApi.Service
}

// API Client that wraps around the LUCI Config client.
func NewApiClient(ctx context.Context) (*apiClient, error) {
	service, err := configApi.NewService(ctx)
	if err != nil {
		return nil, skerr.Fmt("Failed to create new LUCI Config service: %s.", err)
	}
	service.BasePath = API_BASE_PATH
	return &apiClient{service}, nil
}

// Wrapper for LUCI Config's GetProjectConfigs endpoint. Retrieves all
// configs that live in a matching given path..
func (c *apiClient) GetProjectConfigs(path string) ([]*configApi.LuciConfigGetConfigMultiResponseMessageConfigEntry, error) {
	ret, err := c.s.GetProjectConfigs(path).Do()
	if err != nil {
		return nil, skerr.Fmt("Call to GetProjectConfigs failed: %s", err)
	}

	return ret.Configs, nil
}
