// Package gs implements utility for accessing Skia perf data in Google Storage.
package gs

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"cloud.google.com/go/storage"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/util"
	"golang.org/x/net/context"
)

var (
	// dirMap maps dataset name to a slice with Google Storage subdirectory and file prefix.
	dirMap = map[string][]string{
		"skps":  {"pics-json-v2", "bench_"},
		"micro": {"stats-json-v2", "microbench2_"},
	}

	trybotDataPath = regexp.MustCompile(`^[a-z]*[/]?([0-9]{4}/[0-9]{2}/[0-9]{2}/[0-9]{2}/[0-9a-zA-Z-]+-Trybot/[0-9]+/[0-9]+)$`)
)

// lastDate takes a year and month, and returns the last day of the month.
//
// This is done by going to the first day 0:00 of the next month, subtracting an
// hour, then returning the date.
func lastDate(year int, month time.Month) int {
	return time.Date(year, month+1, 1, 0, 0, 0, 0, time.UTC).Add(-time.Hour).Day()
}

// GetLatestGSDirs gets the appropriate directory names in which data
// would be stored between the given timestamp range.
//
// The returning directories cover the range till the date of startTS, and may
// be precise to the hour.
func GetLatestGSDirs(startTS int64, endTS int64, bsSubdir string) []string {
	startTime := time.Unix(startTS, 0).UTC()
	startYear, startMonth, startDay := startTime.Date()
	glog.Infoln("GS dir start time: ", startTime)
	endTime := time.Unix(endTS, 0).UTC()
	lastAddedTime := startTime
	results := make([]string, 0)
	newYear, newMonth, newDay := endTime.Date()
	newHour := endTime.Hour()
	lastYear, lastMonth, _ := lastAddedTime.Date()
	if lastYear != newYear {
		for i := lastYear; i < newYear; i++ {
			if i != startYear {
				results = append(results, fmt.Sprintf("%04d", i))
			} else {
				for j := startMonth; j <= time.December; j++ {
					if j == startMonth && startDay > 1 {
						for k := startDay; k <= lastDate(i, j); k++ {
							results = append(results, fmt.Sprintf("%04d/%02d/%02d", i, j, k))
						}
					} else {
						results = append(results, fmt.Sprintf("%04d/%02d", i, j))
					}
				}
			}
		}
		lastAddedTime = time.Date(newYear, time.January, 1, 0, 0, 0, 0, time.UTC)
	}
	lastYear, lastMonth, _ = lastAddedTime.Date()
	if lastMonth != newMonth {
		for i := lastMonth; i < newMonth; i++ {
			if i != startMonth {
				results = append(results, fmt.Sprintf("%04d/%02d", lastYear, i))
			} else {
				for j := startDay; j <= lastDate(lastYear, i); j++ {
					results = append(results, fmt.Sprintf("%04d/%02d/%02d", lastYear, i, j))
				}
			}
		}
		lastAddedTime = time.Date(newYear, newMonth, 1, 0, 0, 0, 0, time.UTC)
	}
	lastYear, lastMonth, lastDay := lastAddedTime.Date()
	if lastDay != newDay {
		for i := lastDay; i < newDay; i++ {
			results = append(results, fmt.Sprintf("%04d/%02d/%02d", lastYear, lastMonth, i))
		}
		lastAddedTime = time.Date(newYear, newMonth, newDay, 0, 0, 0, 0, time.UTC)
	}
	lastYear, lastMonth, lastDay = lastAddedTime.Date()
	lastHour := lastAddedTime.Hour()
	for i := lastHour; i < newHour+1; i++ {
		results = append(results, fmt.Sprintf("%04d/%02d/%02d/%02d", lastYear, lastMonth, lastDay, i))
	}
	for i := range results {
		results[i] = fmt.Sprintf("%s/%s", bsSubdir, results[i])
	}
	return results
}

// RequestForStorageURL returns an http.Request for a given Cloud Storage URL.
// This is workaround of a known issue: embedded slashes in URLs require use of
// URL.Opaque property
func RequestForStorageURL(url string) (*http.Request, error) {
	r, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("HTTP new request error: %s", err)
	}
	schemePos := strings.Index(url, ":")
	queryPos := strings.Index(url, "?")
	if queryPos == -1 {
		queryPos = len(url)
	}
	r.URL.Opaque = url[schemePos+1 : queryPos]
	return r, nil
}

// FileContentsFromGS returns the contents of a file in the given bucket or an error.
func FileContentsFromGS(s *storage.Client, bucketName, fileName string) ([]byte, error) {
	response, err := s.Bucket(bucketName).Object(fileName).NewReader(context.Background())
	if err != nil {
		return nil, err
	}
	defer util.Close(response)
	return ioutil.ReadAll(response)
}

// AllFilesInDir synchronously iterates through all the files in a given Google Storage folder.
// The callback function is called on each item in the order it is in the bucket.
// It returns an error if the bucket or folder cannot be accessed.
func AllFilesInDir(s *storage.Client, bucket, folder string, callback func(item *storage.ObjectAttrs)) error {
	total := 0
	q := &storage.Query{Prefix: folder, Versions: false}
	for q != nil {
		list, err := s.Bucket(bucket).List(context.Background(), q)
		if err != nil {
			return fmt.Errorf("Problem reading from Google Storage: %v", err)
		}
		total += len(list.Results)
		glog.Infof("Loading %d more files from gs://%s/%s  Total: %d", len(list.Results), bucket, folder, total)
		for _, item := range list.Results {
			callback(item)
		}
		q = list.Next
	}
	return nil
}

// DeleteAllFilesInDir deletes all the files in a given folder.  If processes is set to > 1,
// that many go routines will be spun up to delete the file simultaneously. Otherwise, it will
// be done one one process.
func DeleteAllFilesInDir(s *storage.Client, bucket, folder string, processes int) error {
	if processes <= 0 {
		processes = 1
	}
	errCount := int32(0)
	var wg sync.WaitGroup
	toDelete := make(chan string, 1000)
	for i := 0; i < processes; i++ {
		go deleteHelper(s, bucket, &wg, toDelete, &errCount)
	}
	del := func(item *storage.ObjectAttrs) {
		toDelete <- item.Name
	}
	if err := AllFilesInDir(s, bucket, folder, del); err != nil {
		return err
	}
	close(toDelete)
	wg.Wait()
	if errCount > 0 {
		return fmt.Errorf("There were one or more problems when deleting files in folder %q", folder)
	}
	return nil

}

// deleteHelper spins and waits for work to come in on the toDelete channel.  When it does, it
// uses the storage client to delete the file from the given bucket.
func deleteHelper(s *storage.Client, bucket string, wg *sync.WaitGroup, toDelete <-chan string, errCount *int32) {
	wg.Add(1)
	defer wg.Done()
	for file := range toDelete {
		if err := s.Bucket(bucket).Object(file).Delete(context.Background()); err != nil {
			// Ignore 404 errors on deleting, as they are already gone.
			if !strings.Contains(err.Error(), "statuscode 404") {
				glog.Errorf("Problem deleting gs://%s/%s: %s", bucket, file, err)
				atomic.AddInt32(errCount, 1)
			}
		}
	}
}
