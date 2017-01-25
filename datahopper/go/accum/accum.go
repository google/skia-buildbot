// accum accumulates metrics that were previously reported individually.
package accum

import (
	"sort"
	"strings"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
)

// Reporter is the type of a func that is called to report metrics after they have been accumulated.
type Reporter func(metric string, tags map[string]string, value int64)

// DefaultReporter reports to metrics2.
func DefaultReporter(metric string, tags map[string]string, value int64) {
	metrics2.GetInt64Metric(metric, tags).Update(value)
}

// metricAccum is the information we track per metric trace.
type metricAccum struct {
	total int64
	num   int64
	tags  map[string]string
}

// Accum accumulates measurements with common measurement names and tags.
type Accum struct {
	//     map[<meas>]map[<keys>]
	values map[string]map[string]*metricAccum

	reporter Reporter
}

func New(reporter Reporter) *Accum {
	return &Accum{
		values:   map[string]map[string]*metricAccum{},
		reporter: reporter,
	}
}

func keyFromTags(tags map[string]string) string {
	keys := make([]string, 0, len(tags))
	for key, _ := range tags {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	ret := []string{}
	for _, key := range keys {
		ret = append(ret, key, tags[key])
	}
	return strings.Join(ret, " ")
}

// Add another measurement for 'metric' with 'tags'.
func (a *Accum) Add(metric string, tags map[string]string, duration int64) {
	if _, ok := tags["task-id"]; ok {
		// Log each value to logging to aid in debugging bad bots.
		sklog.Infof("Task: %s %s Value: %d", metric, keyFromTags(tags), duration)
		// Remove 'task-id'.
		delete(tags, "task-id")
	}
	byMetric, ok := a.values[metric]
	if !ok {
		a.values[metric] = map[string]*metricAccum{}
		byMetric = a.values[metric]
	}
	key := keyFromTags(tags)
	byTags, ok := byMetric[key]
	if !ok {
		byMetric[key] = &metricAccum{
			tags: tags,
		}
		byTags = byMetric[key]
	}
	byTags.total += duration
	byTags.num += 1
}

// Report the accumulated metrics and then reset totals and counts.
func (a *Accum) Report() {
	for metric, byTags := range a.values {
		for _, accum := range byTags {
			accum.tags["type"] = "duration"
			a.reporter(metric, accum.tags, accum.total)
			accum.tags["type"] = "num"
			a.reporter(metric, accum.tags, accum.num)
		}
	}
	a.values = map[string]map[string]*metricAccum{}
}
