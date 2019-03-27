package time_window

import (
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

// Parse returns a TimeWindow instance based on the given string. Times are
// interpreted as GMT. The accepted format is as follows:
//
//      FullWindowExpr      = SingleDayWindowExpr(;SingleDayWindowExpr)*
//      SingleDayWindowExpr = DayRangesExpr TimeExpr-TimeExpr
//      DayRangesExpr       = (*|DayRangeExpr(,DayRangeExpr)*)
//      DayRangeExpr        = DayExpr(-DayExpr)?
//      DayExpr             = (Su|M|Tu|W|Th|F|Sa)
//      TimeExpr            = \d\d:\d\d
//
// Examples:
//   Day range:                         M-F 09:00-17:00
//   Every day:                         * 00:00-23:59
//   Multiple days, same time:          Sa,M-W 08:00-09:00
//   Multiple days, different times:    Sa 08:00-09:00; M-W 12:00-03:00
//   Wrap around to next day:           M-F 22:00-02:00
//
func Parse(s string) (*TimeWindow, error) {
	dayWindows := []*dayWindow{}
	split := strings.Split(strings.TrimSpace(s), ";")
	for _, s := range split {
		dw, err := parseDayWindows(s)
		if err != nil {
			return nil, err
		}
		dayWindows = append(dayWindows, dw...)
	}
	return &TimeWindow{
		dayWindows: dayWindows,
	}, nil
}

// dayTime represents a single time of day, specified using hours and minutes.
type dayTime struct {
	hours   int
	minutes int
}

// parse a dayTime from a string formatted like: "02:34"
func parseDayTime(s string) (dayTime, error) {
	var rv dayTime
	split := strings.Split(strings.TrimSpace(s), ":")
	if len(split) != 2 {
		return rv, fmt.Errorf("Expected time format \"hh:mm\", not %q", s)
	}
	for _, comp := range split {
		if len(comp) != 2 {
			return rv, fmt.Errorf("Expected time format \"hh:mm\", not %q", s)
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
	rv.hours = hours
	rv.minutes = minutes
	return rv, nil
}

// dayWindow represents a single window of time.
type dayWindow struct {
	day   time.Weekday
	start dayTime
	end   dayTime
}

// test returns true iff the given time.Time occurs within the dayWindow.
func (w dayWindow) test(t time.Time) bool {
	// Find the nearest start and end to this t.
	start := time.Date(t.Year(), t.Month(), t.Day(), w.start.hours, w.start.minutes, 0, 0, time.UTC)
	for start.Weekday() != w.day {
		start = start.Add(-24 * time.Hour)
	}
	end := time.Date(start.Year(), start.Month(), start.Day(), w.end.hours, w.end.minutes, 0, 0, time.UTC)
	if !start.After(t) && end.After(t) {
		return true
	}
	start = start.Add(7 * 24 * time.Hour)
	end = end.Add(7 * 24 * time.Hour)
	return !start.After(t) && end.After(t)
}

// parse days and dayWindows from a string formatted like: "M-W,Th,Sa 02:34-03:45"
func parseDayWindows(s string) ([]*dayWindow, error) {
	split := strings.SplitN(strings.TrimSpace(s), " ", 2)
	if len(split) != 2 {
		return nil, fmt.Errorf("Expected format \"D hh:mm\", not %q", s)
	}
	dayExpr := strings.TrimSpace(split[0])
	timeExpr := strings.TrimSpace(split[1])

	// Parse the starting and ending times.
	timeSplit := strings.Split(timeExpr, "-")
	if len(timeSplit) != 2 {
		return nil, fmt.Errorf("Expected window format \"hh:mm-hh:mm\", not %q", split[1])
	}
	start, err := parseDayTime(timeSplit[0])
	if err != nil {
		return nil, err
	}
	end, err := parseDayTime(timeSplit[1])
	if err != nil {
		return nil, err
	}
	// If the end time is before the start time, the window rolls over to
	// the next day.
	now := time.Now()
	startTs := time.Date(now.Year(), now.Month(), now.Day(), start.hours, start.minutes, 0, 0, time.UTC)
	endTs := time.Date(now.Year(), now.Month(), now.Day(), end.hours, end.minutes, 0, 0, time.UTC)
	if !endTs.After(startTs) {
		end.hours += 24
	}

	// Parse the day(s).

	// "*" means every day.
	rv := []*dayWindow{}
	if dayExpr == "*" {
		for _, d := range dayMap {
			rv = append(rv, &dayWindow{
				day:   d,
				start: start,
				end:   end,
			})
		}
		return rv, nil
	}

	// We support multiple day expressions.
	daySplit := strings.Split(dayExpr, ",")
	for _, dayExpr := range daySplit {
		rangeSplit := strings.Split(dayExpr, "-")
		if len(rangeSplit) == 1 {
			day, ok := dayMap[dayExpr]
			if !ok {
				return nil, fmt.Errorf("Unknown day %q", dayExpr)
			}
			rv = append(rv, &dayWindow{
				day:   day,
				start: start,
				end:   end,
			})
		} else if len(rangeSplit) == 2 {
			startDay, ok := dayMap[rangeSplit[0]]
			if !ok {
				return nil, fmt.Errorf("Unknown day %q", rangeSplit[0])
			}
			endDay, ok := dayMap[rangeSplit[1]]
			if !ok {
				return nil, fmt.Errorf("Unknown day %q", rangeSplit[1])
			}
			if endDay < startDay {
				endDay += 7
			}
			for i := startDay; i <= endDay; i++ {
				day := time.Weekday(i % 7)
				rv = append(rv, &dayWindow{
					day:   day,
					start: start,
					end:   end,
				})
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
	dayWindows []*dayWindow
}

// Test returns true iff the given time.Time occurs within the TimeWindow.
func (w *TimeWindow) Test(t time.Time) bool {
	if w == nil {
		return true
	}
	t = t.UTC()
	for _, dw := range w.dayWindows {
		if dw.test(t) {
			return true
		}
	}
	return false
}
