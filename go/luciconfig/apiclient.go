package luciconfig

import (
	"context"

	luci_auth "go.chromium.org/luci/auth"
	"go.chromium.org/luci/config"
	luci_config "go.chromium.org/luci/config/impl/remote"
	"go.chromium.org/luci/hardcoded/chromeinfra"
	"go.skia.org/infra/go/skerr"
)

const (
	SERVICE_HOST = "config.luci.app"
)

type ProjectConfig struct {
	Content  string
	Revision string
}

// Interface for LUCI Config wrapper.
type ApiClient interface {
	// Given a LUCI Config path, retrieve all matching configs to that path.
	GetProjectConfigs(ctx context.Context, path string) ([]*ProjectConfig, error)
}

type apiClient struct {
	s config.Interface
}

// API Client that wraps around the LUCI Config client.
func NewApiClient(ctx context.Context) (*apiClient, error) {
	authOpts := chromeinfra.DefaultAuthOptions()
	authOpts.UseIDTokens = true
	authOpts.Audience = "https://" + SERVICE_HOST

	creds, err := luci_auth.NewAuthenticator(ctx, luci_auth.SilentLogin, authOpts).PerRPCCredentials()
	if err != nil {
		return nil, skerr.Fmt("Failed to get credentials to access LUCI Config: %s", err)
	}

	service, err := luci_config.NewV2(ctx, luci_config.V2Options{
		Host:  SERVICE_HOST,
		Creds: creds,
	})
	if err != nil {
		return nil, skerr.Fmt("Failed to create new LUCI Config service: %s.", err)
	}

	return &apiClient{service}, nil
}

// Wrapper for LUCI Config's GetProjectConfigs endpoint. Retrieves all
// configs that live in a matching given path.
func (c *apiClient) GetProjectConfigs(ctx context.Context, path string) ([]*ProjectConfig, error) {
	luciConfigs, err := c.s.GetProjectConfigs(ctx, path, false)
	if err != nil {
		return nil, skerr.Fmt("Call to GetProjectConfigs failed: %s", err)
	}

	projectConfigs := make([]*ProjectConfig, len(luciConfigs))
	for i, luciConfig := range luciConfigs {
		projectConfigs[i] = &ProjectConfig{
			Content:  luciConfig.Content,
			Revision: luciConfig.Meta.Revision,
		}
	}
	return projectConfigs, nil
}
