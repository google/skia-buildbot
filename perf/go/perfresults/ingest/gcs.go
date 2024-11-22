package ingest

import (
	"fmt"
	"time"

	"go.skia.org/infra/perf/go/perfresults"
	"go.skia.org/infra/pinpoint/go/bot_configs"
)

const (
	PublicBucket   = "chrome-perf-public"
	InternalBucket = "chrome-perf-non-public"
)

func convertTime(t time.Time) string {
	t = t.UTC()
	return fmt.Sprintf("%04d/%02d/%02d/%02d", t.Year(), t.Month(), t.Day(), t.Hour())
}

func isInternal(bi perfresults.BuildInfo) bool {
	// It is considered to be external only it is defined as external,
	// otherwise defaults to internal.
	if _, err := bot_configs.GetBotConfig(bi.BuilderName, true); err == nil {
		return false
	}
	return true
}

func convertBuildInfo(bi perfresults.BuildInfo) string {
	mg := bi.MachineGroup
	// Defaults to ChromiumPerf if not set.
	if mg == "" {
		mg = "ChromiumPerf"
	}

	bn := bi.BuilderName
	if bn == "" {
		bn = "BuilderNone"
	}
	return fmt.Sprintf("%s/%s", mg, bn)
}

func convertPath(t time.Time, bi perfresults.BuildInfo, benchmark string) string {
	bucket := PublicBucket
	if isInternal(bi) {
		bucket = InternalBucket
	}
	return fmt.Sprintf("gs://%s/ingest/%s/%s/%s", bucket, convertTime(t), convertBuildInfo(bi), benchmark)
}
