package time_window

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	// dayMap maps abbreviated day name to the index of the day in the week,
	// as defined by the time package.
	dayMap = map[string]time.Weekday{
		"Su": time.Sunday,
		"M":  time.Monday,
		"Tu": time.Tuesday,
		"W":  time.Wednesday,
		"Th": time.Thursday,
		"F":  time.Friday,
		"Sa": time.Saturday,
	}
)

// dayTime represents a single time of day, specified using hours, minutes, and
// seconds.
type dayTime struct {
	hours   int
	minutes int
	seconds int
}

// onDate returns a time.Time which uses the date from the given time.Time and
// the time from this dayTime.
func (dt dayTime) onDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), dt.hours, dt.minutes, dt.seconds, 0, time.UTC)
}

// after returns true if the given Time is after this dayTime.
func (dt dayTime) after(t time.Time) bool {
	return dt.onDate(t).After(t)
}

// parse a dayTime from a string formatted like: "02:34:56"
func parseDayTime(s string) (dayTime, error) {
	var rv dayTime
	split := strings.Split(strings.TrimSpace(s), ":")
	if len(split) != 3 {
		return rv, fmt.Errorf("Expected time format \"hh:mm:ss\", not %q", s)
	}
	for _, comp := range split {
		if len(comp) != 2 {
			return rv, fmt.Errorf("Expected time format \"hh:mm:ss\", not %q", s)
		}
	}
	hours, err := strconv.Atoi(split[0])
	if err != nil {
		return rv, fmt.Errorf("Failed to parse %q as hours: %s", split[0], err)
	}
	if hours < 0 || hours >= 24 {
		return rv, fmt.Errorf("Hours must be between 0-23, not %d", hours)
	}
	minutes, err := strconv.Atoi(split[1])
	if err != nil {
		return rv, fmt.Errorf("Failed to parse %q as minutes: %s", split[1], err)
	}
	if minutes < 0 || minutes >= 60 {
		return rv, fmt.Errorf("Minutes must be between 0-59, not %d", minutes)
	}
	seconds, err := strconv.Atoi(split[2])
	if err != nil {
		return rv, fmt.Errorf("Failed to parse %q as seconds: %s", split[2], err)
	}
	if seconds < 0 || seconds >= 60 {
		return rv, fmt.Errorf("Seconds must be between 0-59, not %d", seconds)
	}
	rv.hours = hours
	rv.minutes = minutes
	rv.seconds = seconds
	return rv, nil
}

// dayWindow represents a single window of time.
type dayWindow struct {
	start dayTime
	end   dayTime
}

// test returns true iff the given time.Time occurs within the dayWindow.
func (w dayWindow) test(t time.Time) bool {
	return !w.start.after(t) && w.end.after(t)
}

// parse a dayWindow from a string formatted like: "02:34:56-03:45:57"
func parseDayWindow(s string) (dayWindow, error) {
	var rv dayWindow
	split := strings.Split(strings.TrimSpace(s), "-")
	if len(split) != 2 {
		return rv, fmt.Errorf("Expected window format \"hh:mm:ss-hh:mm:ss\", not %q", s)
	}
	start, err := parseDayTime(split[0])
	if err != nil {
		return rv, err
	}
	end, err := parseDayTime(split[1])
	if err != nil {
		return rv, err
	}
	now := time.Now()
	startTs := start.onDate(now)
	endTs := end.onDate(now)
	if !endTs.After(startTs) {
		return rv, errors.New("Window end time must be after start time.")
	}
	rv.start = start
	rv.end = end
	return rv, nil
}

// dayWindows represents multiple windows of time.
type dayWindows []dayWindow

// test returns true iff the given time.Time occurs within any of the
// dayWindows.
func (ws dayWindows) test(t time.Time) bool {
	for _, dw := range ws {
		if dw.test(t) {
			return true
		}
	}
	return false
}

// parse dayWindows from a string formatted like: "02:34:56-03:45:57,16:34:56-18:45:57"
func parseDayWindows(s string) (dayWindows, error) {
	split := strings.Split(strings.TrimSpace(s), ",")
	rv := make([]dayWindow, 0, len(split))
	for _, s := range split {
		dw, err := parseDayWindow(s)
		if err != nil {
			return nil, err
		}
		rv = append(rv, dw)
	}
	return rv, nil
}

// parse days and dayWindows from a string formatted like: "M-W,Th,Sa 02:34:56-03:45:57,16:34:56-18:45:57"
func parseDaysWindows(s string) (map[time.Weekday]dayWindows, error) {
	split := strings.SplitN(strings.TrimSpace(s), " ", 2)
	if len(split) != 2 {
		return nil, fmt.Errorf("Expected format \"D hh:mm:ss\", not %q", s)
	}
	dw, err := parseDayWindows(split[1])
	if err != nil {
		return nil, err
	}

	// "*" means every day.
	dayExpr := split[0]
	rv := map[time.Weekday]dayWindows{}
	if dayExpr == "*" {
		for _, d := range dayMap {
			rv[d] = dw
		}
		return rv, nil
	}

	// We support multiple day expressions.
	split = strings.Split(dayExpr, ",")
	for _, dayExpr := range split {
		split2 := strings.Split(dayExpr, "-")
		if len(split2) == 1 {
			day, ok := dayMap[dayExpr]
			if !ok {
				return nil, fmt.Errorf("Unknown day %q", dayExpr)
			}
			rv[day] = dw
		} else if len(split2) == 2 {
			start, ok := dayMap[split2[0]]
			if !ok {
				return nil, fmt.Errorf("Unknown day %q", split2[0])
			}
			end, ok := dayMap[split2[1]]
			if !ok {
				return nil, fmt.Errorf("Unknown day %q", split2[1])
			}
			if end < start {
				end += 7
			}
			for i := start; i <= end; i++ {
				rv[i%7] = dw
			}
		} else {
			return nil, fmt.Errorf("Invalid day expression: %q", dayExpr)
		}
	}
	return rv, nil
}

// TimeWindow specifies a set of time windows on each day of the week in which
// a roller is allowed to upload rolls.
type TimeWindow struct {
	daysWindows map[time.Weekday]dayWindows
}

// Test returns true iff the given time.Time occurs within the TimeWindow.
func (w *TimeWindow) Test(t time.Time) bool {
	if w == nil {
		return true
	}
	t = t.UTC()
	return w.daysWindows[t.Weekday()].test(t)
}

// Parse returns a TimeWindow instance based on the given string. Times are
// interpreted as GMT. The accepted format is as follows:
//
//	FullWindowExpr      = SingleDayExpr(;SingleDayExpr)*
//	SingleDayWindowExpr = DayExpr WindowExpr(,WindowExpr)*
//	DayRangesExpr       = (*|DayRangeExpr(,DayRangeExpr)*)
//	DayRangeExpr        = Day(-Day)?
//	DayExpr             = (Su|M|Tu|W|Th|F|Sa)
//	WindowExpr          = TimeExpr-TimeExpr
//	TimeExpr            = \d\d:\d\d:\d\d
//
// Examples:
//	M-F 09:00:00-17:00:00
//	* 00:00:00-23:59:59
//	Sa,M-W 08:00:00-09:00:00,16:00:00-17:00:00
//
func Parse(s string) (*TimeWindow, error) {
	daysWindows := map[time.Weekday]dayWindows{}
	split := strings.Split(strings.TrimSpace(s), ";")
	for _, s := range split {
		dw, err := parseDaysWindows(s)
		if err != nil {
			return nil, err
		}
		for day, windows := range dw {
			daysWindows[day] = append(daysWindows[day], windows...)
		}
	}
	return &TimeWindow{
		daysWindows: daysWindows,
	}, nil
}
