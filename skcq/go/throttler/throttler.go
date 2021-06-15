package throttler

import (
	"fmt"
	"time"

	"go.skia.org/infra/skcq/go/config"
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
		MaxBurst:       2,
		BurstDelaySecs: 300,
	}
}

// Throttle looks at the specified commit time and determines if the
// commit should be blocked because it violates the throttler config.
// Eg:...
func Throttle(repoBranch string, commitTime time.Time) bool {
	updateThrottler()
	if data, ok := repoBranchToThrottlerData[repoBranch]; ok {
		fmt.Println("THROTTLER IS RETURNING ")
		fmt.Println(len(data.CommitTimes) >= data.ThrottlerCfg.MaxBurst)
		return len(data.CommitTimes) >= data.ThrottlerCfg.MaxBurst
	}
	fmt.Println("THROTTLER IS RETURNING FALSE - NO NEED TO THROTTLE")
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
func updateThrottler() {
	fmt.Println("THROTTLER CLEANING UP BEFORE-")
	fmt.Print("\n%+v\n", repoBranchToThrottlerData)
	for _, data := range repoBranchToThrottlerData {
		// This will be populated with all commits that fit.
		newCommitTimes := []time.Time{}
		cutoffDurationSecs := time.Duration(data.ThrottlerCfg.MaxBurst) * time.Second
		for _, ct := range data.CommitTimes {
			if time.Now().Sub(ct) <= cutoffDurationSecs {
				newCommitTimes = append(newCommitTimes, ct)
			} else {
				fmt.Printf("\nCLEANING UP Timestamp %+v", ct)
			}
		}
		data.CommitTimes = newCommitTimes
	}
	fmt.Println("THROTTLER CLEANING UP LATER-")
	fmt.Print("\n%+v\n", repoBranchToThrottlerData)
}
