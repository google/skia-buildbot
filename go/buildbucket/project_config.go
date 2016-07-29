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

	var rv BuildbucketCfg
	if err := proto.UnmarshalText(buf.String(), &rv); err != nil {
		return nil, err
	}
	return &rv, nil
}

// GetBotsForRepo obtains the list of bots from the given repo by reading the
// cr-buildbucket.cfg file from Gitiles.
func GetBotsForRepo(repo string) ([]string, error) {
	cfg, err := GetProjectConfig(repo)
	if err != nil {
		return nil, err
	}
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
	return bots, nil
}
