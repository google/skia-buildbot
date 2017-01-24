package accum

import (
	"sort"
	"strings"

	"go.skia.org/infra/go/metrics2"
)

type Reporter func(metric string, tags map[string]string, value int64)

func DefaultReporter(metric string, tags map[string]string, value int64) {
	metrics2.GetInt64Metric(metric, tags).Update(value)
}

type metricAccum struct {
	duration    int64
	numDuration int64
	tags        map[string]string
}

// Accum accumulates measurements for the given set of tags
// and measurement name.
//
type Accum struct {
	//       map[<meas>]map[<keys>]
	values   map[string]map[string]*metricAccum
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
	return strings.Join(ret, ":")
}

func (a *Accum) Add(metric string, tags map[string]string, duration int64) {
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
	byTags.duration += duration
	byTags.numDuration += 1
}

func (a *Accum) Report() {
	for metric, byTags := range a.values {
		for _, accum := range byTags {
			accum.tags["type"] = "duration"
			a.reporter(metric, accum.tags, accum.duration)
			accum.tags["type"] = "num"
			a.reporter(metric, accum.tags, accum.numDuration)
		}
	}
	a.values = map[string]map[string]*metricAccum{}
}
