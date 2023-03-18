// Package providers builds different kinds of provider.Provider.
package providers

import (
	"context"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/git/provider"
	"go.skia.org/infra/perf/go/git/providers/git_checkout"
)

// New builds a Provider based on the instance config.
func New(ctx context.Context, instanceConfig *config.InstanceConfig) (provider.Provider, error) {
	if util.In(string(instanceConfig.GitRepoConfig.Provider), []string{"", string(config.GitProviderCLI)}) {
		return git_checkout.New(ctx, instanceConfig)
	}
	return nil, skerr.Fmt("invalid type of Provider selected: %q expected one of %q", instanceConfig.GitRepoConfig.Provider, config.AllGitProviders)
}
