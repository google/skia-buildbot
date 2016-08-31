// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package tsmon

import (
	"golang.org/x/net/context"

	"github.com/luci/gae/service/module"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/tsmon"
	"github.com/luci/luci-go/common/tsmon/metric"
	"github.com/luci/luci-go/common/tsmon/runtimestats"
	"github.com/luci/luci-go/common/tsmon/types"
)

var (
	defaultVersion = metric.NewCallbackString(
		"appengine/default_version",
		"Name of the version currently marked as default.",
		types.MetricMetadata{})
)

// collectGlobalMetrics populates service-global metrics.
//
// Called by tsmon from inside /housekeeping cron handler. Metrics reported must
// not depend on the state of the particular process that happens to report
// them.
func collectGlobalMetrics(c context.Context) {
	version, err := module.Get(c).DefaultVersion("")
	if err != nil {
		logging.Errorf(c, "Error getting default appengine version: %s", err)
		defaultVersion.Set(c, "(unknown)")
	} else {
		defaultVersion.Set(c, version)
	}
}

// collectProcessMetrics populates per-process metrics.
//
// It is called by each individual process right before flushing the metrics.
func collectProcessMetrics(c context.Context, s *tsmonSettings) {
	if s.ReportRuntimeStats {
		runtimestats.Report(c)
	}
}

func init() {
	tsmon.RegisterGlobalCallback(collectGlobalMetrics, defaultVersion)
}
