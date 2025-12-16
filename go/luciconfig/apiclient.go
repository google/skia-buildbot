package luciconfig

import (
	"context"

	"go.chromium.org/luci/config"
	luci_config "go.chromium.org/luci/config/impl/remote"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/skerr"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/idtoken"
	"google.golang.org/api/option"
	"google.golang.org/grpc/credentials/oauth"
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
func NewApiClient(ctx context.Context, local bool) (*apiClient, error) {
	var ts oauth2.TokenSource
	var err error
	if local {
		ts, err = google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail)
	} else {
		credentials, err := google.FindDefaultCredentials(ctx)
		if err != nil {
			return nil, skerr.Fmt("Failed to generate default credentials: %w", err)
		}

		ts, err = idtoken.NewTokenSource(ctx, "https://"+SERVICE_HOST, option.WithCredentials(credentials))
	}

	if err != nil {
		return nil, skerr.Fmt("Failed to get credentials to access LUCI Config: %s", err)
	}

	service, err := luci_config.New(ctx, luci_config.Options{
		Host:      SERVICE_HOST,
		Creds:     oauth.TokenSource{TokenSource: ts},
		UserAgent: "SkiaPerf",
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
