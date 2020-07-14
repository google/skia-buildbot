// Package export has the functionality needed to export results from search
// to JSON. It was at one point used by skqp. That dependency is unclear at present.
package export

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/search/frontend"
	"go.skia.org/infra/golden/go/types"
)

const urlTemplate = "%s/img/images/%s.png"

// DigestInfo contains information about one test result. This include
// the parameter sets.
type DigestInfo struct {
	*frontend.SearchResult        // Same digest information as returned by search results.
	URL                    string // URL from which to retrieve the image.
}

// TestRecord accumulates the images/digests generated for one test.
// This is the format of the meta.json file.
type TestRecord struct {
	TestName types.TestName `json:"testName"`
	Digests  []*DigestInfo  `json:"digests"`
}

// ToTestRecords converts a given search response into a slice of TestRecords.
func ToTestRecords(searchResp *frontend.SearchResponse, imgBaseURL string) []*TestRecord {
	// Group the results by test.
	retMap := map[types.TestName]*TestRecord{}
	for _, oneDigest := range searchResp.Results {
		testNameVal := oneDigest.ParamSet[types.PrimaryKeyField]
		if len(testNameVal) == 0 {
			sklog.Errorf("Error: Digest '%s' has no primaryKey in paramset", oneDigest.Digest)
			continue
		}

		digestInfo := &DigestInfo{
			SearchResult: oneDigest,
			URL:          digestURL(imgBaseURL, oneDigest.Digest),
		}

		testName := types.TestName(oneDigest.ParamSet[types.PrimaryKeyField][0])
		if found, ok := retMap[testName]; ok {
			found.Digests = append(found.Digests, digestInfo)
		} else {
			retMap[testName] = &TestRecord{
				TestName: testName,
				Digests:  []*DigestInfo{digestInfo},
			}
		}
	}

	// Put the records into an array and return them.
	ret := make([]*TestRecord, 0, len(retMap))
	for _, oneTestRec := range retMap {
		ret = append(ret, oneTestRec)
	}

	return ret
}

// WriteTestRecords writes the retrieved information about tests to the given writer JSON.
func WriteTestRecords(testRecs []*TestRecord, writer io.Writer) error {
	return json.NewEncoder(writer).Encode(testRecs)
}

// readTestRecords loads a file with test records.
func readTestRecords(reader io.Reader) ([]*TestRecord, error) {
	var ret []*TestRecord
	if err := json.NewDecoder(reader).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// digestURL returns the URL given a base URL and the digest.
func digestURL(baseURL string, digest types.Digest) string {
	baseURL = strings.TrimRight(baseURL, "/")
	return fmt.Sprintf(urlTemplate, baseURL, digest)
}
