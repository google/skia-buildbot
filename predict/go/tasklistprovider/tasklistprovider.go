package tasklistprovider

import (
	"time"

	swarmingv1 "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/swarming"
)

// Provider queries the Swarming API bots that have failed in the given time frame.
type Provider struct {
	client swarming.ApiClient
}

func New(client swarming.ApiClient) *Provider {
	return &Provider{
		client: client,
	}
}

// Get returns all the tasks in the Skia pool that have failed that were started in the last time duration.
func (p *Provider) Get(since time.Duration) ([]*swarmingv1.SwarmingRpcsTaskRequestMetadata, error) {
	return p.client.ListTasks(time.Now().Add(-1*since), time.Now(), []string{"pool:Skia"}, "completed_failure")
}
