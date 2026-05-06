package chrome

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/trace_visibility/provider"
)

type buildersConfig struct {
	PublicPerfBuilders []string `json:"public_perf_builders"`
}

type chromeProvider struct {
	sources map[string]config.VisibilitySourceConfig
	client  *http.Client
}

// ChromeProvider creates a new Provider instance for Chrome.
func ChromeProvider(cfg config.VisibilityConfig, client *http.Client) (provider.Provider, error) {
	if len(cfg.Sources) == 0 {
		return nil, skerr.Fmt("at least one source is required")
	}

	for name, s := range cfg.Sources {
		if s.GitRepo == "" || s.Path == "" || s.RulePrefix == "" {
			return nil, skerr.Fmt("source %q is missing required fields (git_repo, path, rule_prefix)", name)
		}
	}

	return &chromeProvider{
		sources: cfg.Sources,
		client:  client,
	}, nil
}

// GetExpectedRules retrieves the Chrome visibility config and returns it as data-agnostic rules.
func (f *chromeProvider) GetExpectedRules(ctx context.Context) (map[string]bool, error) {
	allRules := make(map[string]bool)

	for _, s := range f.sources {
		rules, err := f.fetchRulesForSource(ctx, s)
		if err != nil {
			return nil, skerr.Wrapf(err, "failed to fetch rules for source %q", s.GitRepo)
		}
		for r, v := range rules {
			allRules[r] = v
		}
	}

	return allRules, nil
}

func (f *chromeProvider) fetchRulesForSource(ctx context.Context, s config.VisibilitySourceConfig) (map[string]bool, error) {
	repo := gitiles.NewRepo(s.GitRepo, f.client)

	b, err := repo.ReadFile(ctx, s.Path)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to read file %q from gitiles repo %q", s.Path, s.GitRepo)
	}

	var cfg buildersConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, skerr.Wrapf(err, "failed to unmarshal builders config from %q", s.Path)
	}

	rules := make(map[string]bool)
	for _, b := range cfg.PublicPerfBuilders {
		rules[fmt.Sprintf("%s%s", s.RulePrefix, b)] = true
	}

	return rules, nil
}
