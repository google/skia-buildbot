package fuzzpool

import (
	"fmt"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/fuzzer/go/data"
)

func loadReports() *FuzzPool {
	addingOrder := []string{"aaaa", "bbbb", "eeee", "dddd",
		"cccc", "ffff", "gggg", "jjjj"}

	pool := &FuzzPool{}
	for _, key := range addingOrder {
		pool.AddFuzzReport(data.MockReport("skpicture", key))
	}
	addingOrder = []string{"iiii", "hhhh"}
	for _, key := range addingOrder {
		pool.AddFuzzReport(data.MockReport("api", key))
	}
	pool.CurrentFromStaging()
	return pool
}

func TestGetAll(t *testing.T) {
	pool := loadReports()
	actualReports := pool.Reports()

	assert.Equal(t, 10, len(actualReports), "Not all the reports showed up in all filter")
	expectedNameOrder := []string{"aaaa", "bbbb", "cccc", "dddd",
		"eeee", "ffff", "gggg", "hhhh", "iiii", "jjjj"}
	for i, r := range actualReports {
		assert.Equal(t, expectedNameOrder[i], r.FuzzName, fmt.Sprintf("The order was messed up at index %d", i))
	}
}
