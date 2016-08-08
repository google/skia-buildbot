package buildbucket

import (
	"bytes"

	"github.com/golang/protobuf/proto"
	"go.skia.org/infra/go/gitiles"
)

const (
	PROJECT_CFG_FILE    = "cr-buildbucket.cfg"
	INFRA_CONFIG_BRANCH = "infra/config"
)

// GetProjectConfig loads the cr-buildbucket.cfg file from the given repo via Gitiles.
func GetProjectConfig(repo string) (*BuildbucketCfg, error) {
	var buf bytes.Buffer
	if err := gitiles.NewRepo(repo).ReadFileAtRef(PROJECT_CFG_FILE, INFRA_CONFIG_BRANCH, &buf); err != nil {
		return nil, err
	}
	return ParseProjectConfig(buf.String())
}

// GetBotsForConfig obtains the list of bots from the given project config.
func GetBotsForConfig(cfg *BuildbucketCfg) []string {
	bots := []string{}
	for _, bucket := range cfg.Buckets {
		if bucket.Swarming != nil {
			if bucket.Swarming.Builders != nil {
				for _, bot := range bucket.Swarming.Builders {
					bots = append(bots, *bot.Name)
				}
			}
		}
	}
	return bots
}

// GetBotsForRepo obtains the list of bots from the given repo by reading the
// cr-buildbucket.cfg file from Gitiles.
func GetBotsForRepo(repo string) ([]string, error) {
	cfg, err := GetProjectConfig(repo)
	if err != nil {
		return nil, err
	}
	return GetBotsForConfig(cfg), nil
}

// ParseProjectConfig parses the string to obtain a project config.
func ParseProjectConfig(cfg string) (*BuildbucketCfg, error) {
	var rv BuildbucketCfg
	if err := proto.UnmarshalText(cfg, &rv); err != nil {
		return nil, err
	}
	return &rv, nil
}
