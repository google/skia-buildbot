package gatherer

// The gatherer package contains the logic that figures out which powercycle-
// enabled bots or devices are down.
// See the Gatherer interface for more details.

import (
	"context"
	"sort"
	"sync"
	"time"

	swarming "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/am/go/alertclient"
	"go.skia.org/infra/am/go/incident"
	"go.skia.org/infra/go/sklog"
	skswarming "go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/power/go/decider"
	"go.skia.org/infra/power/go/recorder"
	"go.skia.org/infra/skolo/go/powercycle"
)

const (
	// Alert messages from prometheus
	ALERT_BOT_MISSING     = "BotMissing"
	ALERT_BOT_QUARANTINED = "BotQuarantined"

	// Status messages we report.
	STATUS_HOST_MISSING   = "Host Missing"
	STATUS_DEVICE_MISSING = "Device Missing"
)

// Gatherer is a simple interface around the logic behind obtaining
// a list of bots and devices that are down and could be powercycled.
type Gatherer interface {
	// DownBots returns the current set of down bots. It may be cached.
	DownBots() []DownBot
}

// DownBot represents information about a dead or quarantined bot, as well
// as the alert that is related to it.
type DownBot struct {
	BotID      string                                 `json:"bot_id"`
	HostID     string                                 `json:"host_id"`
	Dimensions []*swarming.SwarmingRpcsStringListPair `json:"dimensions"`
	Status     string                                 `json:"status"`
	// Since represents how long the alert been firing
	Since    time.Time `json:"since"`
	Silenced bool      `json:"silenced"`
}

// The gatherer struct implements the Gatherer interface.
type gatherer struct {
	downBots []DownBot
	mutex    sync.Mutex

	iSwarming skswarming.ApiClient
	eSwarming skswarming.ApiClient
	alerts    alertclient.APIClient
	decider   decider.Decider
	hostMap   map[powercycle.DeviceID]string // maps bot id -> jumphost name
	recorder  recorder.Recorder
}

// NewPollingGatherer returns a Gatherer created with the given utilities. all the passed in
// clients should be properly authenticated.
func NewPollingGatherer(ctx context.Context, external, internal skswarming.ApiClient, alerts alertclient.APIClient, decider decider.Decider, recorder recorder.Recorder, hostMap map[powercycle.DeviceID]string, period time.Duration) Gatherer {
	g := &gatherer{
		iSwarming: internal,
		eSwarming: external,
		alerts:    alerts,
		decider:   decider,
		hostMap:   hostMap,
		recorder:  recorder,
	}
	if period > 0 {
		go func() {
			g.update(ctx)
			for {
				<-time.Tick(period)
				g.update(ctx)
			}
		}()
	}
	return g
}

// See the Gatherer interface for more information.
func (g *gatherer) DownBots() []DownBot {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	return g.downBots
}

// set writes to the underlying downBots slice in a thread safe way.
func (g *gatherer) set(bots []DownBot) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	g.downBots = bots
}

// filterDownBots returns a subset of the alert slice that has to
// do with down bots.
func filterDownBots(alerts []incident.Incident) []incident.Incident {
	retVal := []incident.Incident{}
	for _, a := range alerts {
		alertName := string(a.Params["alertname"])
		if alertName == ALERT_BOT_MISSING || alertName == ALERT_BOT_QUARANTINED {
			retVal = append(retVal, a)
		}
	}
	return retVal
}

// update is the "inner loop" of the gatherer. It polls swarming for a list of
// down bots. It then polls alerts for a list of down bots. It constructs the
// intersect of those lists and sets the result in g.downBots.
func (g *gatherer) update(ctx context.Context) {
	// Ask Swarming API for list of bots down in the pools we care about
	sklog.Info("Polling PromAlerts and Swarming API for down bots")
	bots := []*swarming.SwarmingRpcsBotInfo{}
	for _, pool := range skswarming.POOLS_PRIVATE {
		xb, err := g.iSwarming.ListDownBots(ctx, pool)
		if err != nil {
			sklog.Warningf("Could not get down bots from internal pool %s: %s", pool, err)
		}
		bots = append(bots, xb...)
	}

	for _, pool := range skswarming.POOLS_PUBLIC {
		xb, err := g.eSwarming.ListDownBots(ctx, pool)
		if err != nil {
			sklog.Warningf("Could not get down bots from external pool %s: %s", pool, err)
		}
		bots = append(bots, xb...)
	}

	if len(bots) == 0 {
		g.set([]DownBot{})
		sklog.Info("Swarming reports no down bots")
		return
	}

	sklog.Infof("Swarming reports %d down bots", len(bots))

	// Ask Prometheus for bot alerts related to quarantined and dead
	alerts, err := g.alerts.GetAlerts()
	if err != nil {
		sklog.Warningf("Could not get down bots from alerts %s", err)
		return
	}
	alerts = filterDownBots(alerts)

	if len(alerts) == 0 {
		g.set([]DownBot{})
		sklog.Info("No bot-related alerts")
		return
	}
	sklog.Infof("alert manager reports %d alerts", len(alerts))

	silences, err := g.alerts.GetSilences()
	if err != nil {
		sklog.Warningf("Could not get silences from alerts %s", err)
		return
	}

	sklog.Infof("alert manager reports %d silences", len(silences))

	// join these together to create []DownBot
	botsWithAlerts := util.StringSet{}
	alertMap := map[string]incident.Incident{}
	for _, a := range alerts {
		id := string(a.Params["bot"])
		botsWithAlerts[id] = true
		alertMap[id] = a
	}
	botsFromSwarming := util.StringSet{}
	for _, b := range bots {
		botsFromSwarming[b.BotId] = true
	}
	matchingBots := botsWithAlerts.Intersect(botsFromSwarming)

	downBots := []DownBot{}
	for _, b := range bots {
		if unique, ok := matchingBots[b.BotId]; ok && unique {
			alert := alertMap[b.BotId]
			if g.decider.ShouldPowercycleBot(b) {
				downBots = append(downBots, DownBot{
					BotID:      b.BotId,
					HostID:     g.hostMap[powercycle.DeviceID(b.BotId)],
					Dimensions: b.Dimensions,
					Status:     STATUS_HOST_MISSING,
					Since:      time.Unix(alert.Start, 0).UTC(),
					Silenced:   alert.IsSilenced(silences, true),
				})
			} else if g.decider.ShouldPowercycleDevice(b) {
				downBots = append(downBots, DownBot{
					BotID:      b.BotId,
					HostID:     g.hostMap[powercycle.DeviceID(b.BotId+"-device")],
					Dimensions: b.Dimensions,
					Status:     STATUS_DEVICE_MISSING,
					Since:      time.Unix(alert.Start, 0).UTC(),
					Silenced:   alert.IsSilenced(silences, true),
				})
			}
			// Avoid reporting the same bot down more than once
			// This happens when GetDownBots returns a dead and quarantined bot.
			// This assumes that bot ids are unique, even across pools.
			matchingBots[b.BotId] = false
		}
	}

	// Return sorted based on BotID for determinism and organization.
	sort.Slice(downBots, func(i, j int) bool {
		return downBots[i].BotID < downBots[j].BotID
	})

	fixed, broke := g.identifyChangedBots(downBots)

	g.recorder.NewlyFixedBots(fixed)
	g.recorder.NewlyDownBots(broke)

	g.set(downBots)
	sklog.Infof("Done, found %d bots", len(downBots))
}

// identifyChangedBots enumerates the bots that are new since last time and those
//  that were listed and are now fixed.
func (g *gatherer) identifyChangedBots(currentDownBots []DownBot) (fixed []string, broke []string) {
	getName := func(b DownBot) string {
		if b.Status == STATUS_HOST_MISSING {
			return b.BotID
		} else {
			return b.BotID + "-device"
		}
	}
	prev := util.StringSet{}
	for _, b := range g.downBots {
		prev[getName(b)] = true
	}
	curr := util.StringSet{}
	for _, b := range currentDownBots {
		curr[getName(b)] = true
	}

	fixed = prev.Complement(curr).Keys()
	broke = curr.Complement(prev).Keys()

	sort.Strings(fixed)
	sort.Strings(broke)

	return fixed, broke
}
