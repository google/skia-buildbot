// Package human provides human friendly display formats.
package human

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/skia-dev/glog"
)

const MIN_TICKS = 2

// TimeOp is a function that given a time will return a string that would be a
// good label for that time.
type TimeOp func(time.Time) string

// choices is the list of hour increments and associated TimeOps for those
// durations, sorted from largest to smallest hour duration.
var choices = []struct {
	Duration time.Duration
	Op       TimeOp
}{
	{24 * 7 * 4 * time.Hour, func(t time.Time) string { return t.Format("Jan") }},             // Month
	{24 * 3 * time.Hour, func(t time.Time) string { return t.Format("2") + suffix(t.Day()) }}, // Day of month
	{24 * time.Hour, func(t time.Time) string { return t.Format("Mon") }},                     // Weekdays
	{2 * time.Hour, func(t time.Time) string { return t.Format("3pm") }},                      // Hours
}

// suffix returns the correct suffix for a day number.
//
// For making human friendly dates:
//
//  1st, 2nd, 3rd, etc.
//
func suffix(day int) string {
	// The rules are pretty simple, everything ends in "th", except numbers that
	// end in 1, 2 or 3 which end in "st", "nd" and "rd" respectively. The only
	// exceptions are the teens (10 - 19) which always end in "th".
	if day >= 4 && day <= 20 {
		return "th"
	} else if day%10 == 1 {
		return "st"
	} else if day%10 == 2 {
		return "nd"
	} else if day%10 == 3 {
		return "rd"
	}
	return "th"
}

// opFromHours takes a number of hours and from that returns a function that
// will produce good labels for tick marks in that time range. For example, if
// the time range is small enough then the ticks will be marked with the
// weekday, e.g.  "Sun", if the time range is much larger the ticks may be
// marked with the month, e.g. "Jul".
func opFromHours(duration time.Duration) (TimeOp, error) {
	// Move down the list of choices from the largest granularity to the finest.
	// The first one that would generate more than MIN_TICKS for the given number
	// of hours is chosen and that TimeOp is returned.
	var Op TimeOp = nil
	for _, c := range choices {
		if duration > c.Duration {
			Op = c.Op
			break
		}
	}
	if Op == nil {
		return nil, fmt.Errorf("Couldn't calculate TimeOp")
	}
	return Op, nil
}

// Tick represents a single tick mark.
type Tick struct {
	X     float64
	Value string
}

// ToFlot converts a slice of Ticks into something that will serialize into
// JSON that Flot consumes, which is an array of 2 element arrays. The 2 element
// arrays contain an x offset and then a label as a string. For example:
//
//   [ [ 0.5, "Saturday" ], [ 1.5, "Sunday" ] ]
//
func ToFlot(ticks []*Tick) []interface{} {
	ret := []interface{}{}
	for _, t := range ticks {
		ret = append(ret, []interface{}{t.X, t.Value})
	}
	return ret
}

// TickMarks produces human readable tickmarks to span the given timestamps.
//
// The array of timestamps are presumed to be in ascending order.
// If 'in' is nil then the "Local" time zone is used.
//
// The choice of how to label the tick marks is based on the full time
// range of the timestamps.
func TickMarks(timestamps []int64, in *time.Location) []*Tick {
	loc := in
	if loc == nil {
		var err error
		loc, err = time.LoadLocation("Local")
		if err != nil {
			loc = time.UTC
		}
	}
	ret := []*Tick{}
	if len(timestamps) < 2 {
		glog.Warning("Insufficient number of commits: %d", len(timestamps))
		return ret
	}

	begin := time.Unix(timestamps[0], 0).In(loc)
	end := time.Unix(timestamps[len(timestamps)-1], 0).In(loc)
	duration := end.Sub(begin)
	op, err := opFromHours(duration)
	if err != nil {
		glog.Errorf("Failed to calculate tickmarks for: %s %s: %s", begin, end, err)
		return ret
	}
	last := op(begin)
	ret = append(ret, &Tick{X: 0, Value: last})
	for i, t := range timestamps {
		if tickValue := op(time.Unix(t, 0).In(loc)); last != tickValue {
			last = tickValue
			ret = append(ret, &Tick{X: float64(i) - 0.5, Value: tickValue})
		}
	}

	return ret
}

// FlotTickMarks returns a struct that will serialize into JSON that Flot
// expects for a value for tick marks.
//
// If an error occurs the tick list will be empty.
func FlotTickMarks(ts []int64) []interface{} {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		glog.Errorf("Failed to load the timezone: %s", err)
		return []interface{}{}
	}
	return ToFlot(TickMarks(ts, loc))
}

var durationRe = regexp.MustCompile("([0-9]+)([smhdw])$")

// ParseDuration parses a human readable duration. Note that this understands
// both days and weeks, which time.ParseDuration does not support.
func ParseDuration(s string) (time.Duration, error) {
	parsed := durationRe.FindStringSubmatch(s)
	if len(parsed) != 3 {
		return time.Duration(0), fmt.Errorf("Invalid format: %s", s)
	}
	n, err := strconv.ParseInt(parsed[1], 10, 32)
	if err != nil {
		return time.Duration(0), fmt.Errorf("Invalid numeric format: %s", s)
	}
	d := time.Second
	switch parsed[2][0] {
	case 's':
		d = time.Duration(n) * time.Second
	case 'm':
		d = time.Duration(n) * time.Minute
	case 'h':
		d = time.Duration(n) * time.Hour
	case 'd':
		d = time.Duration(n) * 24 * time.Hour
	case 'w':
		d = time.Duration(n) * 7 * 24 * time.Hour
	}
	return d, nil
}
