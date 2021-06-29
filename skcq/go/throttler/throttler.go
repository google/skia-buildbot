package throttler

import (
	"time"

	"go.skia.org/infra/skcq/go/config"
)

const (
	MaxBurstDefault       = 2
	BurstDelaySecsDefault = 300
)

var (
	repoBranchToThrottlerData = map[string]*ThrottlerData{}
)

type ThrottlerData struct {
	CommitTimes  []time.Time          `json:"commit_times"`
	ThrottlerCfg *config.ThrottlerCfg `json:"throttler_cfg"`
}

func GetDefaultThrottlerCfg() *config.ThrottlerCfg {
	return &config.ThrottlerCfg{
		MaxBurst:       MaxBurstDefault,
		BurstDelaySecs: BurstDelaySecsDefault,
	}
}

// Throttle looks at the specified commit time and determines if the
// commit should be blocked because it violates the throttler config.
// Eg:...
func Throttle(repoBranch string, commitTime time.Time) bool {
	cleanupThrottler()
	if data, ok := repoBranchToThrottlerData[repoBranch]; ok {
		return len(data.CommitTimes) >= data.ThrottlerCfg.MaxBurst
	}
	return false
}

// Everytime the throttler is updated it is cleaned up.
func UpdateThrottler(repoBranch string, commitTime time.Time, throttlerCfg *config.ThrottlerCfg) {
	if data, ok := repoBranchToThrottlerData[repoBranch]; !ok {
		if throttlerCfg == nil {
			throttlerCfg = GetDefaultThrottlerCfg()
		}
		repoBranchToThrottlerData[repoBranch] = &ThrottlerData{
			CommitTimes:  []time.Time{commitTime},
			ThrottlerCfg: throttlerCfg,
		}
	} else {
		data.CommitTimes = append(data.CommitTimes, commitTime)
	}
}

// cleanupThrottler goes through the throttler data and removes all entries older than the burst
// burst delay cutoff.
func cleanupThrottler() {
	for _, data := range repoBranchToThrottlerData {
		// This will be populated with all commits that fit.
		newCommitTimes := []time.Time{}
		cutoffDurationSecs := time.Duration(data.ThrottlerCfg.BurstDelaySecs) * time.Second
		for _, ct := range data.CommitTimes {
			if time.Now().Sub(ct) <= cutoffDurationSecs {
				newCommitTimes = append(newCommitTimes, ct)
			}
		}
		data.CommitTimes = newCommitTimes
	}
}
