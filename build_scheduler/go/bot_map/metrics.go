package bot_map

import (
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/buildbot"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/swarming"
)

/*
	Metrics for bot_map. Used for warning when the bot_map is out of date or does not match reality.
*/

const (
	MEASUREMENT_BOTMAP_BUILDERS_IS_MAPPED     = "botmap.builders.is-mapped"
	MEASUREMENT_BOTMAP_BUILDERS_MAPPED_EXISTS = "botmap.builders.mapped-exists"
	MEASUREMENT_BOTMAP_BUILDERS_OVERLAP       = "botmap.builders.overlap"
	MEASUREMENT_BOTMAP_TRIGGERS_IS_MAPPED     = "botmap.triggers.is-mapped"
	MEASUREMENT_BOTMAP_TRIGGERS_MAPPED_EXISTS = "botmap.triggers.mapped-exists"
)

func StartMetrics(swarm swarming.ApiClient) {
	go func() {
		for _ = range time.Tick(time.Minute) {
			// Ensure that the Swarming trigger bots in bot_map match reality.
			actualSwarmTriggers, err := swarm.ListSkiaTriggerBots()
			if err != nil {
				glog.Error(err)
				continue
			}
			// Determine whether each trigger bot is mapped in bot_map.
			for _, actualTriggerBot := range actualSwarmTriggers {
				mapped := 0
				if _, ok := BUILDERS_BY_SWARMING_BOT[actualTriggerBot.BotId]; ok {
					mapped = 1
				}
				metrics2.GetInt64Metric(MEASUREMENT_BOTMAP_TRIGGERS_IS_MAPPED, map[string]string{"bot": actualTriggerBot.BotId}).Update(int64(mapped))
			}

			// Determine whether each trigger bot in bot_map still exists.
			m := make(map[string]bool, len(actualSwarmTriggers))
			for _, b := range actualSwarmTriggers {
				m[b.BotId] = true
			}
			for mappedBot, _ := range BUILDERS_BY_SWARMING_BOT {
				exists := 0
				if _, ok := m[mappedBot]; ok {
					exists = 1
				}
				metrics2.GetInt64Metric(MEASUREMENT_BOTMAP_TRIGGERS_MAPPED_EXISTS, map[string]string{"bot": mappedBot}).Update(int64(exists))
			}
		}
	}()

	go func() {
		for _ = range time.Tick(time.Minute) {
			// Ensure that the builders in bot_map match those specified in cr-buildbucket.cfg.
			cfgBuilders, err := buildbucket.GetBotsForRepo("https://skia.googlesource.com/skia")
			if err != nil {
				glog.Error(err)
				continue
			}
			m := make(map[string]bool, len(cfgBuilders))
			for _, b := range cfgBuilders {
				m[b] = true
			}
			// Determine whether each builder in the project config is mapped in bot_map.
			for _, cfgBuilder := range cfgBuilders {
				mapped := 0
				if _, ok := SWARMING_BOTS_BY_BUILDER[cfgBuilder]; ok {
					mapped = 1
				}
				metrics2.GetInt64Metric(MEASUREMENT_BOTMAP_BUILDERS_IS_MAPPED, map[string]string{"builder": cfgBuilder}).Update(int64(mapped))
			}

			// Determine whether each builder in bot_map exists in the project config.
			for mappedBuilder, _ := range SWARMING_BOTS_BY_BUILDER {
				exists := 0
				if _, ok := m[mappedBuilder]; ok {
					exists = 1
				}
				metrics2.GetInt64Metric(MEASUREMENT_BOTMAP_BUILDERS_MAPPED_EXISTS, map[string]string{"builder": mappedBuilder}).Update(int64(exists))
			}
		}
	}()

	go func() {
		for _ = range time.Tick(time.Minute) {
			// Ensure that the builders in bot_map don't overlap with Buildbot builders.
			buildbotBuilders, err := buildbot.GetBuilders()
			if err != nil {
				glog.Error(err)
				continue
			}
			cfgBuilders, err := buildbucket.GetBotsForRepo("https://skia.googlesource.com/skia")
			if err != nil {
				glog.Error(err)
				continue
			}
			m := make(map[string]bool, len(cfgBuilders))
			for _, b := range cfgBuilders {
				m[b] = true
			}

			for b, _ := range buildbotBuilders {
				overlap := 0
				if _, ok := m[b]; ok {
					overlap = 1
				}
				metrics2.GetInt64Metric(MEASUREMENT_BOTMAP_BUILDERS_OVERLAP, map[string]string{"source": "buildbot", "builder": b}).Update(int64(overlap))
			}

			for b, _ := range m {
				overlap := 0
				if _, ok := buildbotBuilders[b]; ok {
					overlap = 1
				}
				metrics2.GetInt64Metric(MEASUREMENT_BOTMAP_BUILDERS_OVERLAP, map[string]string{"source": "project-config", "builder": b}).Update(int64(overlap))
			}
		}
	}()
}
