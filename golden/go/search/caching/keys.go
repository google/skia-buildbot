package caching

import (
	"fmt"

	"go.skia.org/infra/golden/go/sql/schema"
)

func ByBlameKey(corpus string) string {
	return corpus + "_byblame"
}

// MatchingUntriagedTracesKey returns a key to be used to cache the data for untriaged traces.
func MatchingUntriagedTracesKey(corpus string) string {
	return fmt.Sprintf("matchingTraces_%s_untriaged", corpus)
}

// MatchingPositiveTracesKey returns a key to be used to cache the data for positive traces.
func MatchingPositiveTracesKey(corpus string) string {
	return fmt.Sprintf("matchingTraces_%s_positive", corpus)
}

// MatchingNegativeTracesKey returns a key to be used to cache the data for negative traces.
func MatchingNegativeTracesKey(corpus string) string {
	return fmt.Sprintf("matchingTraces_%s_negative", corpus)
}

// MatchingIgnoredTracesKey returns a key to be used to cache the data for ignored traces.
func MatchingIgnoredTracesKey(corpus string) string {
	return fmt.Sprintf("matchingTraces_%s_ignored", corpus)
}

// DigestsForGroupingKey returns a key to be used to cache the digests for grouping data.
func DigestsForGroupingKey(groupingID schema.GroupingID, traces string) string {
	return fmt.Sprintf("digestsForGroup_%s_%s", groupingID, traces)
}
