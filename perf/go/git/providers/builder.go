// Package providers builds different kinds of provider.Provider.
package providers

import (
	"context"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/git/provider"
	"go.skia.org/infra/perf/go/git/providers/git_checkout"
	"go.skia.org/infra/perf/go/git/providers/gitiles"
	"golang.org/x/oauth2/google"
)

// New builds a Provider based on the instance config.
func New(ctx context.Context, instanceConfig *config.InstanceConfig) (provider.Provider, error) {
	prov := instanceConfig.GitRepoConfig.Provider

	if util.In(string(prov), []string{"", string(config.GitProviderCLI)}) {
		return git_checkout.New(ctx, instanceConfig)
	} else if prov == config.GitProviderGitiles {
		client, err := google.DefaultClient(ctx, auth.ScopeGerrit)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		return gitiles.New(client, instanceConfig), nil
	}
	return nil, skerr.Fmt("invalid type of Provider selected: %q expected one of %q", instanceConfig.GitRepoConfig.Provider, config.AllGitProviders)
}
