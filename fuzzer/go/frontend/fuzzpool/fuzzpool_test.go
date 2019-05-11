package fuzzpool

import (
	"fmt"
	"testing"

	assert "github.com/stretchr/testify/require"
	fcommon "go.skia.org/infra/fuzzer/go/common"
	"go.skia.org/infra/fuzzer/go/data"
	"go.skia.org/infra/go/testutils/unittest"
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
	unittest.SmallTest(t)
	pool := loadReports()
	actualReports := pool.Reports()

	assert.Equal(t, 10, len(actualReports), "Not all the reports showed up in all filter")
	expectedNameOrder := []string{"aaaa", "bbbb", "cccc", "dddd",
		"eeee", "ffff", "gggg", "hhhh", "iiii", "jjjj"}
	for i, r := range actualReports {
		assert.Equal(t, expectedNameOrder[i], r.FuzzName, fmt.Sprintf("The order was messed up at index %d", i))
	}
}

func TestGetByName(t *testing.T) {
	unittest.SmallTest(t)
	pool := loadReports()
	r, err := pool.FindFuzzDetailForFuzz("bbbb")
	assert.NoError(t, err)

	assert.Equal(t, "bbbb", r.FuzzName)
	assert.Equal(t, "skpicture", r.FuzzCategory)
	assert.Equal(t, "mock_arm8", r.FuzzArchitecture)
	assert.Equal(t, "mock/package/alpha", r.FileName)
	assert.Equal(t, "beta", r.FunctionName)
	assert.Equal(t, 16, r.LineNumber)
}

func TestGetByCategory(t *testing.T) {
	unittest.SmallTest(t)
	pool := loadReports()
	actualReports, err := pool.FindFuzzDetails("skpicture", "", "", "", "", fcommon.UNKNOWN_LINE)
	assert.NoError(t, err)

	assert.Equal(t, 8, len(actualReports), "Not all the reports showed up in all filter")
	expectedNameOrder := []string{"aaaa", "bbbb", "cccc", "dddd",
		"eeee", "ffff", "gggg", "jjjj"}
	for i, r := range actualReports {
		assert.Equal(t, expectedNameOrder[i], r.FuzzName, "The order was messed up at index %d", i)
	}

	actualReports, err = pool.FindFuzzDetails("api", "", "", "", "", fcommon.UNKNOWN_LINE)
	assert.NoError(t, err)

	assert.Equal(t, 2, len(actualReports), "Not all the reports showed up in all filter")
	expectedNameOrder = []string{"hhhh", "iiii"}
	for i, r := range actualReports {
		assert.Equal(t, expectedNameOrder[i], r.FuzzName, "The order was messed up at index %d", i)
	}
}

func TestGetByArchitecture(t *testing.T) {
	unittest.SmallTest(t)
	pool := loadReports()
	actualReports, err := pool.FindFuzzDetails("", "mock_arm8", "", "", "", fcommon.UNKNOWN_LINE)
	assert.NoError(t, err)

	assert.Equal(t, 8, len(actualReports), "Not all the reports showed up in all filter")
	expectedNameOrder := []string{"aaaa", "bbbb", "cccc", "dddd",
		"eeee", "ffff", "gggg", "iiii"}
	for i, r := range actualReports {
		assert.Equal(t, expectedNameOrder[i], r.FuzzName, "The order was messed up at index %d", i)
	}

	actualReports, err = pool.FindFuzzDetails("skpicture", "mock_x64", "", "", "", fcommon.UNKNOWN_LINE)
	assert.NoError(t, err)

	assert.Equal(t, 1, len(actualReports), "Not all the reports showed up in all filter")
	expectedNameOrder = []string{"jjjj"}
	for i, r := range actualReports {
		assert.Equal(t, expectedNameOrder[i], r.FuzzName, "The order was messed up at index %d", i)
	}
}
func TestGetByBadness(t *testing.T) {
	unittest.SmallTest(t)
	pool := loadReports()
	actualReports, err := pool.FindFuzzDetails("", "", "bad", "", "", fcommon.UNKNOWN_LINE)
	assert.NoError(t, err)

	assert.Equal(t, 9, len(actualReports), "Not all the reports showed up in all filter")
	expectedNameOrder := []string{"aaaa", "bbbb", "cccc", "dddd", "eeee",
		"ffff", "gggg", "hhhh", "iiii"}
	for i, r := range actualReports {
		assert.Equal(t, expectedNameOrder[i], r.FuzzName, "The order was messed up at index %d", i)
	}

	actualReports, err = pool.FindFuzzDetails("skpicture", "mock_x64", "grey", "", "", fcommon.UNKNOWN_LINE)
	assert.NoError(t, err)

	assert.Equal(t, 1, len(actualReports), "Not all the reports showed up in all filter")
	expectedNameOrder = []string{"jjjj"}
	for i, r := range actualReports {
		assert.Equal(t, expectedNameOrder[i], r.FuzzName, "The order was messed up at index %d", i)
	}
}

func TestGetByFilename(t *testing.T) {
	unittest.SmallTest(t)
	pool := loadReports()
	actualReports, err := pool.FindFuzzDetails("", "", "", "mock/package/alpha", "", fcommon.UNKNOWN_LINE)
	assert.NoError(t, err)

	assert.Equal(t, 7, len(actualReports), "Not all the reports showed up in all filter")
	expectedNameOrder := []string{"aaaa", "bbbb", "cccc",
		"ffff", "hhhh", "iiii", "jjjj"}
	for i, r := range actualReports {
		assert.Equal(t, expectedNameOrder[i], r.FuzzName, "The order was messed up at index %d", i)
	}

	actualReports, err = pool.FindFuzzDetails("skpicture", "mock_x64", "grey", "mock/package/alpha", "", fcommon.UNKNOWN_LINE)
	assert.NoError(t, err)

	assert.Equal(t, 1, len(actualReports), "Not all the reports showed up in all filter")
	expectedNameOrder = []string{"jjjj"}
	for i, r := range actualReports {
		assert.Equal(t, expectedNameOrder[i], r.FuzzName, "The order was messed up at index %d", i)
	}
}

func TestGetByFunctionName(t *testing.T) {
	unittest.SmallTest(t)
	pool := loadReports()
	actualReports, err := pool.FindFuzzDetails("", "", "", "", "beta", fcommon.UNKNOWN_LINE)
	assert.NoError(t, err)

	assert.Equal(t, 6, len(actualReports), "Not all the reports showed up in all filter")
	expectedNameOrder := []string{"aaaa", "bbbb",
		"ffff", "hhhh", "iiii", "jjjj"}
	for i, r := range actualReports {
		assert.Equal(t, expectedNameOrder[i], r.FuzzName, "The order was messed up at index %d", i)
	}

	actualReports, err = pool.FindFuzzDetails("skpicture", "mock_arm8", "bad", "mock/package/delta", "epsilon", fcommon.UNKNOWN_LINE)
	assert.NoError(t, err)

	assert.Equal(t, 2, len(actualReports), "Not all the reports showed up in all filter")
	expectedNameOrder = []string{"dddd", "gggg"}
	for i, r := range actualReports {
		assert.Equal(t, expectedNameOrder[i], r.FuzzName, "The order was messed up at index %d", i)
	}
}

func TestGetByLineNumber(t *testing.T) {
	unittest.SmallTest(t)
	pool := loadReports()
	actualReports, err := pool.FindFuzzDetails("", "", "", "", "", 16)
	assert.NoError(t, err)

	assert.Equal(t, 6, len(actualReports), "Not all the reports showed up in all filter")
	expectedNameOrder := []string{"aaaa", "bbbb",
		"ffff", "hhhh", "iiii", "jjjj"}
	for i, r := range actualReports {
		assert.Equal(t, expectedNameOrder[i], r.FuzzName, "The order was messed up at index %d", i)
	}

	actualReports, err = pool.FindFuzzDetails("skpicture", "mock_arm8", "bad", "mock/package/delta", "epsilon", 122)
	assert.NoError(t, err)

	assert.Equal(t, 1, len(actualReports), "Not all the reports showed up in all filter")
	expectedNameOrder = []string{"gggg"}
	for i, r := range actualReports {
		assert.Equal(t, expectedNameOrder[i], r.FuzzName, "The order was messed up at index %d", i)
	}
}
