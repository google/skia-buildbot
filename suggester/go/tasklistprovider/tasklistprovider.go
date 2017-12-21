package tasklistprovider

import (
	"time"

	swarmingv1 "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/swarming"
)

type Provider struct {
	client swarming.ApiClient
}

func NewProvider(client swarming.ApiClient) *Provider {
	return &Provider{
		client: client,
	}
}

func (p *Provider) Get(since time.Duration) ([]*swarmingv1.SwarmingRpcsTaskRequestMetadata, error) {
	return p.client.ListTasks(time.Now().Add(-1*since), time.Now(), []string{"pool:Skia"}, "completed_failure")
}
