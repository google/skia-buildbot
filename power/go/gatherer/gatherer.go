package gatherer

import (
	"sort"
	"sync"
	"time"

	swarming "github.com/luci/luci-go/common/api/swarming/swarming/v1"
	"github.com/skia-dev/glog"

	"go.skia.org/infra/go/promalertsclient"
	"go.skia.org/infra/go/sklog"
	skswarming "go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/power/go/decider"
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
}

type gatherer struct {
	downBots []DownBot
	mutex    sync.Mutex

	iSwarming skswarming.ApiClient
	eSwarming skswarming.ApiClient
	alerts    promalertsclient.APIClient
	decider   decider.Decider
}

type Gatherer interface {
	GetDownBots() []DownBot
	CycleEvery(time.Duration)
}

func New(external, internal skswarming.ApiClient, alerts promalertsclient.APIClient, decider decider.Decider) Gatherer {
	return &gatherer{
		iSwarming: internal,
		eSwarming: external,
		alerts:    alerts,
		decider:   decider,
	}
}

func (g *gatherer) GetDownBots() []DownBot {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	return g.downBots
}

func (g *gatherer) CycleEvery(d time.Duration) {
	go func() {
		g.cycle()
		for {
			<-time.Tick(d)
			g.cycle()
		}
	}()
}

func (g *gatherer) update(bots []DownBot) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	g.downBots = bots
}

func downBotsFilter(a promalertsclient.Alert) bool {
	alertName := string(a.Labels["alertname"])
	return alertName == ALERT_BOT_MISSING || alertName == ALERT_BOT_QUARANTINED
}

func (g *gatherer) cycle() {
	// Ask Swarming API for list of bots down in the pools we care about
	sklog.Infoln("Polling PromAlerts and Swarming API for down bots")
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
		glog.Info("Swarming reports no down bots")
		return
	}

	sklog.Infof("Swarming reports %d down bots: %s", len(bots), bots)

	// Ask Prometheus for bot alerts related to quarantined and dead
	alerts, err := g.alerts.GetAlerts(downBotsFilter)
	if err != nil {
		sklog.Warningf("Could not get down bots from alerts %s", err)
		return
	}

	if len(alerts) == 0 {
		g.update([]DownBot{})
		glog.Info("No bot-related alerts")
		return
	}

	sklog.Infof("Promalerts reports %d bot-related alerts: %s", len(alerts), alerts)

	// join these together to create []DownBot
	botsWithAlerts := util.NewStringSet()
	alertMap := map[string]promalertsclient.Alert{}
	for _, a := range alerts {
		id := string(a.Labels["bot"])
		botsWithAlerts.Add(id)
		alertMap[id] = a
	}
	botsFromSwarming := util.NewStringSet()
	for _, b := range bots {
		botsFromSwarming.Add(b.BotId)
	}
	matchingBots := botsWithAlerts.Intersect(botsFromSwarming)

	downBots := []DownBot{}
	for _, b := range bots {
		if _, ok := matchingBots[b.BotId]; ok {
			alert := alertMap[b.BotId]
			if g.decider.ShouldPowercycleBot(b) {
				downBots = append(downBots, DownBot{
					BotID:      b.BotId,
					Dimensions: b.Dimensions,
					Status:     STATUS_HOST_MISSING,
					Since:      alert.StartsAt,
					Silenced:   alert.Silenced,
				})
			} else if g.decider.ShouldPowercycleDevice(b) {
				downBots = append(downBots, DownBot{
					BotID:      b.BotId,
					Dimensions: b.Dimensions,
					Status:     STATUS_DEVICE_MISSING,
					Since:      alert.StartsAt,
					Silenced:   alert.Silenced,
				})
			}
		}
	}

	// Return sorted based on BotID for determinism and organization.
	sort.Slice(downBots, func(i, j int) bool {
		return downBots[i].BotID < downBots[j].BotID
	})
	g.update(downBots)
	glog.Infof("Done, found %d bots", len(downBots))
}
