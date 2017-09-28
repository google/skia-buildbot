package battery_usage

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.skia.org/infra/go/sklog"
)

type BatteryUsageResult struct {
	TestName       string
	ChargeConsumed float32 // milli-amphours
	PeakCurrent    float32 // milli-amperes
	Duration       time.Duration
}

type ParsedTest struct {
	Name  string
	Start time.Time
	End   time.Time
}

type BatteryLine struct {
	TS      time.Time
	Current float32
	Voltage float32
}

var BATTERY_LINE = regexp.MustCompile(`(?P<timestamp>\d+) (?P<current>\d\.\d+) (?P<voltage>-?\d.\d+)`)

func parseBatteryFile(content string, sampleHz int) []BatteryLine {
	parsed := []BatteryLine{}
	lines := strings.Split(content, "\n")
	step := int64(time.Second) / int64(sampleHz)

	isFirstSection := true
	lastTimestamp := int64(-1)
	nanos := int64(0)
	for _, line := range lines {
		// Skip comments
		if strings.HasPrefix(line, "#") {
			continue
		}

		if c := BATTERY_LINE.FindStringSubmatch(line); c != nil {
			ts, err := strconv.ParseInt(c[1], 10, 64)
			if err != nil {
				sklog.Warningf("Could not parse int64 %s from %q: %s", c[1], line, err)
				continue
			}
			if isFirstSection && lastTimestamp == -1 {
				lastTimestamp = ts
			}
			if isFirstSection && ts == lastTimestamp {
				// Skip first section because we don't know exactly when it starts
				continue
			} else {
				isFirstSection = false
			}

			if ts != lastTimestamp {
				lastTimestamp = ts
				nanos = 0
			}
			current, err := strconv.ParseFloat(c[2], 32)
			if err != nil {
				sklog.Warningf("Could not parse float %s from %q: %s", c[2], line, err)
				continue
			}
			voltage, err := strconv.ParseFloat(c[3], 32)
			if err != nil {
				sklog.Warningf("Could not parse float %s from %q: %s", c[3], line, err)
				continue
			}
			parsed = append(parsed, BatteryLine{
				TS:      time.Unix(ts, nanos),
				Current: float32(current),
				Voltage: float32(voltage),
			})

			// Step forward
			nanos += step
		}
	}
	return parsed
}

func getBatteryUsage(lines []BatteryLine, test ParsedTest, sampleHz int) BatteryUsageResult {
	// assume lines is sorted by timestamp
	stepDuration := time.Duration(int64(time.Second) / int64(sampleHz))
	stepSeconds := float32(1.0) / float32(sampleHz)

	max := float32(-1.0)
	total := float32(0.0)

	wayBefore := test.Start.Add(-stepDuration)
	lastStep := test.End.Add(-stepDuration)

	for _, line := range lines {
		if line.TS.Before(wayBefore) {
			continue
		}
		// Check to see if we are in the partial rectangle at the beginning of the test.
		if line.TS.Before(test.Start) {
			// Subtract the time from the next step
			width := line.TS.Add(stepDuration).Sub(test.Start)
			if width > 0 {
				if line.Current > max {
					max = line.Current
				}
			}
			total += line.Current * float32(width) / float32(time.Second)
			continue
		}
		// Check to see if we are in the partial rectangle at the end of the test.
		if line.TS.After(lastStep) {
			width := test.End.Sub(line.TS)
			if width > 0 {
				if line.Current > max {
					max = line.Current
				}
			}
			total += line.Current * float32(width) / float32(time.Second)
			break
		}
		// We are in the center of the test, we can just compute easy rectangles
		total += line.Current * stepSeconds
		if line.Current > max {
			max = line.Current
		}
	}

	// convert amps to milliamps
	max = max * 1000.0

	return BatteryUsageResult{
		TestName:       test.Name,
		ChargeConsumed: ampsecondsToMilliAmpHours(total),
		PeakCurrent:    max,
		Duration:       test.End.Sub(test.Start),
	}
}

func ampsecondsToMilliAmpHours(ampSeconds float32) float32 {
	return ampSeconds * 1000.0 / 3600.0
}
