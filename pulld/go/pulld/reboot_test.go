package main

import (
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
)

func TestTimeForReboot(t *testing.T) {
	test := func(now time.Time, expectedGood bool, expectedTime time.Time) {
		actualGood, actualTime := timeForReboot(now)
		assert.Equal(t, expectedGood, actualGood)
		if !expectedGood {
			assert.True(t, expectedTime.Equal(actualTime))
		}
	}
	// Too early.
	expectedTime := time.Date(2016, 10, 20, 5, 0, 0, 0, time.UTC)
	test(time.Date(2016, 10, 20, 0, 0, 0, 0, time.UTC), false, expectedTime)
	test(time.Date(2016, 10, 20, 3, 45, 0, 0, time.UTC), false, expectedTime)
	test(time.Date(2016, 10, 20, 4, 59, 59, 999999999, time.UTC), false, expectedTime)
	// In range.
	test(time.Date(2016, 10, 20, 5, 0, 0, 0, time.UTC), true, time.Time{})
	test(time.Date(2016, 10, 20, 5, 1, 0, 0, time.UTC), true, time.Time{})
	test(time.Date(2016, 10, 20, 5, 59, 59, 999999999, time.UTC), true, time.Time{})
	// Too late.
	expectedTime = time.Date(2016, 10, 21, 5, 0, 0, 0, time.UTC)
	test(time.Date(2016, 10, 20, 6, 0, 0, 0, time.UTC), false, expectedTime)
	test(time.Date(2016, 10, 20, 12, 34, 56, 789, time.UTC), false, expectedTime)
	test(time.Date(2016, 10, 20, 23, 59, 59, 999999999, time.UTC), false, expectedTime)
}
