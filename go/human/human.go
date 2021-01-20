// Package human provides human friendly display formats.
package human

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.skia.org/infra/go/sklog"
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
	{2 * time.Minute, func(t time.Time) string { return t.Format("03:04pm") }},                // Minutes
	{2 * time.Second, func(t time.Time) string { return t.Format("03:04:05pm") }},             // Seconds
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
		sklog.Warningf("Insufficient number of commits: %d", len(timestamps))
		return ret
	}

	begin := time.Unix(timestamps[0], 0).In(loc)
	end := time.Unix(timestamps[len(timestamps)-1], 0).In(loc)
	duration := end.Sub(begin)
	op, err := opFromHours(duration)
	if err != nil {
		sklog.Errorf("Failed to calculate tickmarks for: %s %s: %s", begin, end, err)
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
//
// tz is the timezone, and can be the empty string if the default (Eastern) timezone is acceptable.
func FlotTickMarks(ts []int64, tz string) []interface{} {
	if tz == "" {
		tz = "America/New_York"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc, err = time.LoadLocation("UTC")
		if err != nil {
			sklog.Errorf("Failed to load the timezone %q: %s", tz, err)
			return []interface{}{}
		}
	}
	return ToFlot(TickMarks(ts, loc))
}

const durationTmpl = `\s*([0-9]+)\s*([smhdw])\s*`

var durationRe = regexp.MustCompile(`^(?:` + durationTmpl + `)+$`)
var durationSubRe = regexp.MustCompile(durationTmpl)

// ParseDuration parses a human readable duration. Note that this understands
// both days and weeks, which time.ParseDuration does not support.
func ParseDuration(s string) (time.Duration, error) {
	ret := time.Duration(0)
	if !durationRe.MatchString(s) {
		return ret, fmt.Errorf("Invalid format: %s", s)
	}
	parsed := durationSubRe.FindAllStringSubmatch(s, -1)
	if len(parsed) == 0 {
		return ret, fmt.Errorf("Invalid format: %s", s)
	}
	for _, match := range parsed {
		if len(match) != 3 {
			return ret, fmt.Errorf("Invalid format: %s", s)
		}
		n, err := strconv.ParseInt(match[1], 10, 32)
		if err != nil {
			return ret, fmt.Errorf("Invalid numeric format: %s", s)
		}
		switch match[2][0] {
		case 's':
			ret += time.Duration(n) * time.Second
		case 'm':
			ret += time.Duration(n) * time.Minute
		case 'h':
			ret += time.Duration(n) * time.Hour
		case 'd':
			ret += time.Duration(n) * 24 * time.Hour
		case 'w':
			ret += time.Duration(n) * 7 * 24 * time.Hour
		}
	}
	return ret, nil
}

type delta struct {
	units string
	delta int64
}

var (
	deltas = []delta{
		{
			units: "y",
			delta: 365 * 24 * 60 * 60,
		},
		{
			units: "w",
			delta: 7 * 24 * 60 * 60,
		},
		{
			units: "d",
			delta: 24 * 60 * 60,
		},
		{
			units: "h",
			delta: 60 * 60,
		},
		{
			units: "m",
			delta: 60,
		},
		{
			units: "s",
			delta: 1,
		},
	}
)

// Duration returns a human friendly description of the given time.Duration.
//
// For example Duration(61*time.Second) returns " 1m 1s".
//
// The length of the string returned is guaranteed to always be 7.
// A negative duration is treated the same as a positive duration.
func Duration(duration time.Duration) string {
	ret := []string{}
	s := int64(math.Abs(duration.Seconds()))
	for _, d := range deltas {
		if d.delta <= s {
			ret = append(ret, fmt.Sprintf("%2d%s", s/d.delta, d.units))
			s = s % d.delta
			if len(ret) == 2 {
				break
			}
		}
	}
	if len(ret) == 0 {
		ret = append(ret, "   ", " 0s")
	}
	if len(ret) < 2 {
		ret = []string{"   ", ret[0]}
	}
	return strings.Join(ret, " ")
}

// JSONDuration is a type that implements the json.Unmarshal interface and can be used
// to parse human readable durations from configuration files.
type JSONDuration time.Duration

func (d *JSONDuration) String() string {
	return strings.TrimSpace(Duration(time.Duration(*d)))
}

func (d *JSONDuration) UnmarshalJSON(durBytes []byte) error {
	durStr := strings.Trim(string(durBytes), "\"")
	duration, err := ParseDuration(string(durStr))
	if err != nil {
		return err
	}
	*d = JSONDuration(duration)
	return nil
}
