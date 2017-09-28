package battery_usage

import (
	"testing"
	"time"

	"go.skia.org/infra/go/testutils"

	assert "github.com/stretchr/testify/require"
)

func TestParsingBatteryConsumption(t *testing.T) {
	testutils.SmallTest(t)
	sampleInput := testutils.MustReadFile("battery_consumption.out")
	sampleHertz := 10
	lines := parseBatteryFile(sampleInput, sampleHertz)

	assert.Len(t, lines, 20, "The first samples should be skipped. There are a full 2 seconds of data at 10 hertz, so 20 samples.")

	// spot check
	assert.Equal(t, lines[0], BatteryLine{
		TS:      time.Unix(1500000001, int64(0*time.Millisecond/time.Nanosecond)),
		Current: 0.200000,
		Voltage: -3.9,
	})

	assert.Equal(t, lines[4], BatteryLine{
		TS:      time.Unix(1500000001, int64(400*time.Millisecond/time.Nanosecond)),
		Current: 0.500000,
		Voltage: -3.95,
	})

	assert.Equal(t, lines[10], BatteryLine{
		TS:      time.Unix(1500000002, int64(0*time.Millisecond/time.Nanosecond)),
		Current: 0.500000,
		Voltage: -3.9,
	})

	assert.Equal(t, lines[13], BatteryLine{
		TS:      time.Unix(1500000002, int64(300*time.Millisecond/time.Nanosecond)),
		Current: 0.100000,
		Voltage: -3.9,
	})
}

func TestCombiningEasy(t *testing.T) {
	testutils.SmallTest(t)
	// This test does easy math where the test lines up precisely with the sample points

	lines := []BatteryLine{}
	for i := 0; i < 10; i++ {
		lines = append(lines, BatteryLine{
			TS:      time.Unix(1500000001, int64(time.Duration(i*100)*time.Millisecond/time.Nanosecond)),
			Current: 0.1 * float32(i),
			Voltage: -3.9,
		})
	}

	test := ParsedTest{
		Name:  "my-test",
		Start: time.Unix(1500000001, int64(200*time.Millisecond/time.Nanosecond)),
		End:   time.Unix(1500000001, int64(600*time.Millisecond/time.Nanosecond)),
	}

	sampleHertz := 10
	usage := getBatteryUsage(lines, test, sampleHertz)

	assert.Equal(t, "my-test", usage.TestName)
	assert.InDelta(t, ampsecondsToMilliAmpHours(.14), usage.ChargeConsumed, .000001, "area under the curve, left-side rectangles, 100ms * (.2+.3+.4+.5 A)")
	assert.InDelta(t, 500, usage.PeakCurrent, .000001, "The last rectangle has 600 mA of charge")
	assert.Equal(t, 400*time.Millisecond, usage.Duration)
}

func TestCombiningHard(t *testing.T) {
	testutils.SmallTest(t)
	// This test does extrapolation where the test is a bit wider and has some partial "rectangles"
	// of data

	lines := []BatteryLine{}
	for i := 0; i < 10; i++ {
		lines = append(lines, BatteryLine{
			TS:      time.Unix(1500000001, int64(time.Duration(i*100)*time.Millisecond/time.Nanosecond)),
			Current: 0.1 * float32(i),
			Voltage: -3.9,
		})
	}

	test := ParsedTest{
		Name:  "my-test",
		Start: time.Unix(1500000001, int64(130*time.Millisecond/time.Nanosecond)),
		End:   time.Unix(1500000001, int64(660*time.Millisecond/time.Nanosecond)),
	}

	sampleHertz := 10
	usage := getBatteryUsage(lines, test, sampleHertz)

	assert.Equal(t, "my-test", usage.TestName)
	assert.InDelta(t, ampsecondsToMilliAmpHours(.183), usage.ChargeConsumed, .000001, "area under the curve, left-side rectangles, 100ms * (.2+.3+.4+.5 A) + 70ms * .1 A + 60ms * .6 A")
	assert.InDelta(t, 600, usage.PeakCurrent, .000001, "The last rectangle has 600 mA of charge")
	assert.Equal(t, 530*time.Millisecond, usage.Duration)
}
