package gatherer

import (
	"sort"
	"sync"
	"time"

	swarming "github.com/luci/luci-go/common/api/swarming/swarming/v1"
	"github.com/prometheus/common/model"

	"go.skia.org/infra/go/promalertsclient"
	"go.skia.org/infra/go/sklog"
	skswarming "go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
)

const (
	// Alert messages from prometheus
	ALERT_BOT_MISSING     = "BotMissing"
	ALERT_BOT_QUARANTINED = "BotQuarantined"

	// Status messages we report.
	STATUS_HOST_MISSING   = "Host Missing"
	STATUS_DEVICE_MISSING = "Device Missing"
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
	decider   BotDecider
}

type Gatherer interface {
	GetDownBots() ([]DownBot, error)
}

// The BotDecider interface abstracts away a configuration file or similar which indicates which bots have powercyclable devices attached and which are golo machines, etc
type BotDecider interface {
	ShouldPowercycleBot(*swarming.SwarmingRpcsBotInfo) bool
	ShouldPowercycleDevice(*swarming.SwarmingRpcsBotInfo) bool
	GetBugURL(*swarming.SwarmingRpcsBotInfo) string
}

func New(external, internal skswarming.ApiClient, alerts promalertsclient.APIClient, decider BotDecider) Gatherer {
	return &gatherer{
		iSwarming: internal,
		eSwarming: external,
		alerts:    alerts,
		decider:   decider,
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

func downBotsFilter(a model.Alert) bool {
	alertName := string(a.Labels["alertname"])
	return alertName == ALERT_BOT_MISSING || alertName == ALERT_BOT_QUARANTINED
}

func (g *gatherer) cycle() {
	// Ask Swarming API for list of bots down in the pools we care about

	bots := []*swarming.SwarmingRpcsBotInfo{}
	for _, pool := range skswarming.POOLS_PRIVATE {
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
	botsWithAlerts := util.NewStringSet()
	for _, a := range alerts {
		botsWithAlerts.Add(string(a.Labels["bot"]))
	}
	botsFromSwarming := util.NewStringSet()
	for _, b := range bots {
		botsFromSwarming.Add(b.BotId)
	}
	matchingBots := botsWithAlerts.Intersect(botsFromSwarming)

	downBots := []DownBot{}
	for _, b := range bots {
		if _, ok := matchingBots[b.BotId]; ok {
			if g.decider.ShouldPowercycleBot(b) {
				downBots = append(downBots, DownBot{
					BotID:      b.BotId,
					Dimensions: b.Dimensions,
					Status:     STATUS_HOST_MISSING,
					// FIXME Since:      ,
					BugUrl: g.decider.GetBugURL(b),
				})
			} else if g.decider.ShouldPowercycleDevice(b) {
				downBots = append(downBots, DownBot{
					BotID:      transformBotIDToDevice(b.BotId),
					Dimensions: b.Dimensions,
					Status:     STATUS_DEVICE_MISSING,
					// FIXME Since:      ,
					BugUrl: g.decider.GetBugURL(b),
				})
			}
		}
	}

	// Return sorted based on BotID for determinism and organization.
	sort.Slice(downBots, func(i, j int) bool {
		return downBots[i].BotID < downBots[j].BotID
	})
	// set []DownBot to variable
	g.mutex.Lock()
	defer g.mutex.Unlock()
	g.downBots = downBots
}

func transformBotIDToDevice(id string) string {
	return id + "-device"
}
