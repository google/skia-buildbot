// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package tumble

import (
	"fmt"
	"strings"
	"time"

	"github.com/luci/luci-go/common/clock/clockflag"
	"github.com/luci/luci-go/server/settings"
	"golang.org/x/net/context"
)

const (
	// baseName is the base tumble name. It is also the name of the tumble task
	// queue.
	baseName = "tumble"

	baseURL             = "/internal/" + baseName
	fireAllTasksURL     = baseURL + "/fire_all_tasks"
	processShardPattern = baseURL + "/process_shard/:shard_id/at/:timestamp"
)

// Config is the set of tweakable things for tumble. If you use something other
// than the defaults (e.g. unset values), you must ensure that all aspects of
// your application use the same config.
//
// The JSON annotations are for settings module storage (see settings.go).
type Config struct {
	// NumShards is the number of tumble shards that will process concurrently.
	// It defaults to 32.
	NumShards uint64 `json:"numShards,omitempty"`

	// TemporalMinDelay is the minimum number of seconds to wait before the
	// task queue entry for a given shard will run. It defaults to 1 second.
	TemporalMinDelay clockflag.Duration `json:"temporalMinDelay,omitempty"`

	// TemporalRoundFactor is the number of seconds to batch together in task
	// queue tasks. It defaults to 4 seconds.
	TemporalRoundFactor clockflag.Duration `json:"temporalRoundFactor,omitempty"`

	// DustSettleTimeout is the amount of time to wait in between mutation
	// processing iterations.
	//
	// This should be chosen as a compromise between higher expectations of the
	// eventually-consistent datastore and task processing latency.
	DustSettleTimeout clockflag.Duration `json:"dustSettleTimeout,omitempty"`

	// NumGoroutines is the number of gorountines that will process in parallel
	// in a single shard. Each goroutine will process exactly one root entity.
	// It defaults to 16.
	NumGoroutines int `json:"numGoroutines,omitempty"`

	// ProcessMaxBatchSize is the number of mutations that each processor
	// goroutine will attempt to include in each commit.
	//
	// It defaults to 128. A negative value means no limit.
	ProcessMaxBatchSize int32 `json:"processMaxBatchSize,omitempty"`

	// DelayedMutations enables the 'DelayedMutation' mutation subtype.
	//
	// If you set this to true, you MUST also add the second index mentioned
	// in the package docs.
	DelayedMutations bool `json:"delayedMutations,omitempty"`

	// Namespaced, if true, indicates that Tumble is operating with multitenancy.
	// In this mode, all Tumble operations are namespaced, and Tumble will use
	// a reserved namespace (TaskNamespace) for its own tasks.
	Namespaced bool
}

// defaultConfig returns the default configuration settings.
var defaultConfig = Config{
	NumShards:           32,
	TemporalMinDelay:    clockflag.Duration(time.Second),
	TemporalRoundFactor: clockflag.Duration(4 * time.Second),
	NumGoroutines:       16,
	ProcessMaxBatchSize: 128,
	DustSettleTimeout:   clockflag.Duration(2 * time.Second),
}

// getConfig returns the current configuration.
//
// It first tries to load it from settings. If no settings is installed, or if
// there is no configuration in settings, defaultConfig is returned.
func getConfig(c context.Context) *Config {
	cfg := Config{}
	switch err := settings.Get(c, baseName, &cfg); err {
	case nil:
		break
	case settings.ErrNoSettings:
		// Defaults.
		cfg = defaultConfig
	default:
		panic(fmt.Errorf("could not fetch Tumble settings - %s", err))
	}
	return &cfg
}

// processURL creates a new url for a process shard taskqueue task, including
// the given timestamp and shard number.
func processURL(ts timestamp, shard uint64) string {
	return strings.NewReplacer(
		":shard_id", fmt.Sprint(shard),
		":timestamp", fmt.Sprint(ts)).Replace(processShardPattern)
}
