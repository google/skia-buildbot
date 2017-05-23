package gatherer

import (
	"fmt"
	"sync"
	"time"

	swarming "github.com/luci/luci-go/common/api/swarming/swarming/v1"
	"github.com/prometheus/alertmanager/dispatch"

	"go.skia.org/infra/go/promalertsclient"
	"go.skia.org/infra/go/sklog"
	skswarming "go.skia.org/infra/go/swarming"
)

type DownBot struct {
	BotID      string                                 `json:"bot_id"`
	Dimensions []*swarming.SwarmingRpcsStringListPair `json:"dimensions"`
	Status     string                                 `json:"status"`
	Since      time.Time                              `json:"since"`
	Silenced   bool                                   `json:"silenced"`
	BugUrl     string                                 `json:"bug_url"`
}

type gatherer struct {
	downBots []DownBot
	mutex    sync.Mutex

	iSwarming skswarming.ApiClient
	eSwarming skswarming.ApiClient
	alerts    promalertsclient.APIClient
}

type Gatherer interface {
	GetDownBots() ([]DownBot, error)
}

func New(external, internal skswarming.ApiClient, alerts promalertsclient.APIClient) Gatherer {
	return &gatherer{
		iSwarming: internal,
		eSwarming: external,
		alerts:    alerts,
	}
}

func (g *gatherer) GetDownBots() ([]DownBot, error) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	return g.downBots, nil
}

func (g *gatherer) update(bots []DownBot) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	g.downBots = bots
}

func downBotsFilter(a dispatch.APIAlert) bool {
	alertName := string(a.Labels["alertname"])
	return alertName == "BotMissing" || alertName == "BotQuarantined"
}

func (g *gatherer) cycle() {
	// Ask Swarming API for list of bots down in the pools we care about

	bots := []*swarming.SwarmingRpcsBotInfo{}

	for _, pool := range skswarming.POOLS_PRIVATE {
		fmt.Printf("")
		xb, err := g.iSwarming.ListDownBots(pool)
		if err != nil {
			sklog.Warningf("Could not get down bots from internal pool %s: %s", pool, err)
		}
		bots = append(bots, xb...)
	}

	for _, pool := range skswarming.POOLS_PUBLIC {
		xb, err := g.eSwarming.ListDownBots(pool)
		if err != nil {
			sklog.Warningf("Could not get down bots from external pool %s: %s", pool, err)
		}
		bots = append(bots, xb...)
	}

	if len(bots) == 0 {
		g.update([]DownBot{})
		return
	}

	// Ask Prometheus for bot alerts related to quarantined and dead
	alerts, err := g.alerts.GetAlerts(downBotsFilter)
	if err != nil {
		sklog.Warningf("Could not get down bots from alerts %s", err)
		return
	}

	if len(alerts) == 0 {
		g.update([]DownBot{})
		return
	}

	// join these together to create []DownBot

	downBots := []DownBot{}
	for _, b := range bots {
		downBots = append(downBots, DownBot{
			BotID:      b.BotId,
			Dimensions: b.Dimensions,
			Status:     b.State, //FIXME
			// FIXME Since:      ,
			BugUrl: "",
		})
	}
	// set []DownBot to variable
	g.mutex.Lock()
	defer g.mutex.Unlock()
	g.downBots = downBots
}
