package fileutil

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetHourlyDirs_LargeTimeSpan_ReturnsSortedList(t *testing.T) {
	startTS := time.Date(1972, time.November, 29, 13, 45, 20, 67, time.UTC)
	endTS := time.Date(1982, time.February, 2, 3, 45, 20, 67, time.UTC)
	folders := GetHourlyDirs("prefix", startTS, endTS)
	assert.IsIncreasing(t, folders)
	assert.Len(t, folders, 80439)
	assert.Equal(t, folders[0], "prefix/1972/11/29/13")
	assert.Equal(t, folders[43], "prefix/1972/12/01/08") // spot check
	assert.Equal(t, folders[len(folders)-1], "prefix/1982/02/02/03")
}

func TestGetHourlyDirs_ExactlyOnTheHour_Success(t *testing.T) {
	startTS := time.Date(1985, time.November, 20, 12, 00, 00, 00, time.UTC)
	endTS := time.Date(1985, time.November, 20, 15, 00, 00, 00, time.UTC)
	folders := GetHourlyDirs("prefix", startTS, endTS)
	assert.Equal(t, []string{
		"prefix/1985/11/20/12", "prefix/1985/11/20/13", "prefix/1985/11/20/14", "prefix/1985/11/20/15",
	}, folders)
}

func TestGetHourlyDirs_LessThanOneHourApart_ReturnsOneFolder(t *testing.T) {
	// Test boundaries within an hour.
	startTS := time.Date(1985, time.November, 20, 13, 00, 00, 00, time.UTC)
	endTS := time.Date(1985, time.November, 20, 13, 01, 00, 00, time.UTC)
	folders := GetHourlyDirs("prefix", startTS, endTS)
	assert.Equal(t, []string{"prefix/1985/11/20/13"}, folders)
}

func TestGetHourlyDirs_StartAfterEnd_ReturnsNothing(t *testing.T) {
	startTS := time.Date(2014, time.November, 20, 13, 00, 00, 00, time.UTC)
	endTS := time.Date(1985, time.November, 20, 13, 01, 00, 00, time.UTC)
	folders := GetHourlyDirs("prefix", startTS, endTS)
	assert.Empty(t, folders)
}

func TestGetHourlyDirs_PrefixEmpty_ReturnValueDoesNotStartWithSlash(t *testing.T) {
	startTS := time.Date(2022, time.May, 31, 13, 01, 00, 00, time.UTC)
	endTS := time.Date(2022, time.June, 1, 4, 01, 00, 00, time.UTC)
	folders := GetHourlyDirs("", startTS, endTS)
	assert.Equal(t, []string{
		"2022/05/31/13", "2022/05/31/14", "2022/05/31/15", "2022/05/31/16", "2022/05/31/17",
		"2022/05/31/18", "2022/05/31/19", "2022/05/31/20", "2022/05/31/21", "2022/05/31/22",
		"2022/05/31/23", "2022/06/01/00", "2022/06/01/01", "2022/06/01/02", "2022/06/01/03",
		"2022/06/01/04",
	}, folders)
}
