package flaky

import "time"

type TimeRange struct {
	Begin time.Time
	End   time.Time
}

// In returns true if the timestamp fits within the half-open
// interface of TimeRange, i.e. ts in [Begin, End).
func (t *TimeRange) In(ts time.Time) bool {
	return (ts.Equal(ts) || ts.After(t.Begin)) && ts.Before(t.End)
}

type Flaky map[string][]*TimeRange

func (f Flaky) WasFlaky(botname string, ts time.Time) bool {
	if ranges, ok := f[botname]; ok {
		for _, tr := range ranges {
			if tr.In(ts) {
				return true
			}
		}
	}
	return false
}
