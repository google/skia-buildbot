package search

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/types"
)

const urlTemplate = "%s/img/images/%s.png"

// ExportDigestInfo contains information about one test result. This include
// the parameter sets.
type ExportDigestInfo struct {
	*SRDigest        // Same digest information as returned by search results.
	URL       string // URL from which to retrieve the image.
}

// ExportTestRecord accumulates the images/digests generated for one test.
// This is the format of the meta.json file.
type ExportTestRecord struct {
	TestName types.TestName      `json:"testName"`
	Digests  []*ExportDigestInfo `json:"digests"`
}

// GetExportRecords converts a given search response into a slice of ExportTestRecords.
func GetExportRecords(searchResp *NewSearchResponse, imgBaseURL string) []*ExportTestRecord {
	// Group the results by test.
	retMap := map[types.TestName]*ExportTestRecord{}
	for _, oneDigest := range searchResp.Digests {
		testNameVal := oneDigest.ParamSet[types.PRIMARY_KEY_FIELD]
		if len(testNameVal) == 0 {
			sklog.Errorf("Error: Digest '%s' has no primaryKey in paramset", oneDigest.Digest)
			continue
		}

		digestInfo := &ExportDigestInfo{
			SRDigest: oneDigest,
			URL:      DigestUrl(imgBaseURL, oneDigest.Digest),
		}

		testName := types.TestName(oneDigest.ParamSet[types.PRIMARY_KEY_FIELD][0])
		if found, ok := retMap[testName]; ok {
			found.Digests = append(found.Digests, digestInfo)
		} else {
			retMap[testName] = &ExportTestRecord{
				TestName: testName,
				Digests:  []*ExportDigestInfo{digestInfo},
			}
		}
	}

	// Put the records into an array and return them.
	ret := make([]*ExportTestRecord, 0, len(retMap))
	for _, oneTestRec := range retMap {
		ret = append(ret, oneTestRec)
	}

	return ret
}

// WriteExportTestRecordsFile writes the retrieved information about tests to a file as JSON.
func WriteExportTestRecordsFile(testRecs []*ExportTestRecord, outputPath string) error {
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer util.Close(f)
	return WriteExportTestRecords(testRecs, f)
}

// WriteTestRecords writes the retrieved information about tests to the given writer JSON.
func WriteExportTestRecords(testRecs []*ExportTestRecord, writer io.Writer) error {
	return json.NewEncoder(writer).Encode(testRecs)
}

// ReadExportTestRecords loads a file with test records.
func ReadExportTestRecords(reader io.Reader) ([]*ExportTestRecord, error) {
	ret := []*ExportTestRecord{}
	if err := json.NewDecoder(reader).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// GetURL returns the URL given a base URL and the digest.
func DigestUrl(baseURL string, digest types.Digest) string {
	baseURL = strings.TrimRight(baseURL, "/")
	return fmt.Sprintf(urlTemplate, baseURL, digest)
}
