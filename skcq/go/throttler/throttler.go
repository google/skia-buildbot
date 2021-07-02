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
	// timeNowFunc allows tests to mock out time.Now() for testing.
	timeNowFunc = time.Now
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

// ThrottlerImpl implements the types.ThrottlerManager interface.
// NOTE: This implementation is not thread safe.
type ThrottlerImpl struct {
	repoBranchToThrottlerData map[string]*ThrottlerData
}

// GetThrottler returns an instance of ThrottlerImpl.
func NewThrottler() *ThrottlerImpl {
	return &ThrottlerImpl{
		repoBranchToThrottlerData: map[string]*ThrottlerData{},
	}
}

// Throttle implements the types.ThrottlerManager interface.
func (t *ThrottlerImpl) Throttle(repoBranch string, commitTime time.Time) bool {
	t.cleanupThrottler()
	if data, ok := t.repoBranchToThrottlerData[repoBranch]; ok {
		return len(data.CommitTimes) >= data.ThrottlerCfg.MaxBurst
	}
	return false
}

// UpdateThrottler implements the types.ThrottlerManager interface.
func (t *ThrottlerImpl) UpdateThrottler(repoBranch string, commitTime time.Time, throttlerCfg *config.ThrottlerCfg) {
	if data, ok := t.repoBranchToThrottlerData[repoBranch]; !ok {
		if throttlerCfg == nil {
			throttlerCfg = GetDefaultThrottlerCfg()
		}
		t.repoBranchToThrottlerData[repoBranch] = &ThrottlerData{
			CommitTimes:  []time.Time{commitTime},
			ThrottlerCfg: throttlerCfg,
		}
	} else {
		data.CommitTimes = append(data.CommitTimes, commitTime)
	}
}

// cleanupThrottler goes through the throttler data and removes all entries
// older than the burst delay cutoff.
func (t *ThrottlerImpl) cleanupThrottler() {
	for _, data := range t.repoBranchToThrottlerData {
		// This will be populated with all commits that fit the throttle window.
		newCommitTimes := []time.Time{}
		cutoffDurationSecs := time.Duration(data.ThrottlerCfg.BurstDelaySecs) * time.Second
		for _, ct := range data.CommitTimes {
			if timeNowFunc().Sub(ct) <= cutoffDurationSecs {
				newCommitTimes = append(newCommitTimes, ct)
			}
		}
		data.CommitTimes = newCommitTimes
	}
}
