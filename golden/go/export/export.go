// package export defines structure and functions to export data
// from Gold.
package export

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/search"
)

const urlTemplate = "%s/img/images/%s.png"

// DigestInfo contains information about one test result. This include
// the parameter sets.
type DigestInfo struct {
	*search.SRDigest        // Same digest information as returned by search results.
	URL              string // URL from which to retrieve the image.
}

// TestRecord accumulates the images/digests generated for one test.
// This is the format of the meta.json file.
type TestRecord struct {
	TestName string        `json:"testName"`
	Digests  []*DigestInfo `json:"digests"`
}

// WriteTestRecords writes the retrieved information about tests to disk as JSON.
func WriteTestRecords(outputPath string, testRecs []*TestRecord) error {
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer util.Close(f)

	return json.NewEncoder(f).Encode(testRecs)
}

// ReadTestRecords loads a file with test records.
func ReadTestRecords(inputPath string) ([]*TestRecord, error) {
	f, err := os.Open(inputPath)
	if err != nil {
		return nil, err
	}
	defer util.Close(f)

	ret := []*TestRecord{}
	if err := json.NewDecoder(f).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// GetURL returns the URL given a base URL and the digest.
func DigestUrl(baseURL, digest string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	return fmt.Sprintf(urlTemplate, baseURL, digest)
}
