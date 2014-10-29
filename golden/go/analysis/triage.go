package analysis

import (
	"sort"

	"github.com/golang/glog"

	"skia.googlesource.com/buildbot.git/golden/go/types"
)

// GUITestDetails is an output type with triage information grouped by tests.
type GUITestDetails map[string]*GUITestDetail

// GUITestDetail contains the untriaged, positive and negative digests of
// a test with all the information necessary to triage the digests.
type GUITestDetail struct {
	Untriaged map[string]*GUIUntriagedDigest `json:"untriaged"`
	Positive  map[string]*DigestInfo         `json:"positive"`
	Negative  map[string]*DigestInfo         `json:"negative"`
}

// DigestInfo contains the image URL and the occurence count of a digest.
type DigestInfo struct {
	ImgUrl string `json:"imgUrl"`
	Count  int    `json:"count"`
}

// GUIUntriagedDigest is an output type for a single digest to be triaged.
// Aside from the digests image url and params it also contains metrics
// comparing it to the positive digests.
type GUIUntriagedDigest struct {
	ImgUrl    string              `json:"imgUrl"`
	ParamsSet []map[string]string `json:"paramsSet"`
	Diffs     GUIDiffMetrics      `json:"diffs"`
}

// GUIDiffMetrics is a sortable slice of diff metrics.
type GUIDiffMetrics []*GUIDiffMetric

func (m GUIDiffMetrics) Len() int { return len(m) }
func (m GUIDiffMetrics) Less(i, j int) bool {
	return (m[i] == nil) || (m[j] == nil) || (m[i].PixelDiffPercent < m[j].PixelDiffPercent)
}
func (m GUIDiffMetrics) Swap(i, j int) { m[i], m[j] = m[j], m[i] }

// GUIDiffMetric is an output type to store the diff metrics comparing an
// untriaged digest to a positive digest as well as the url of the diff image.
type GUIDiffMetric struct {
	NumDiffPixels    int     `json:"numDiffPixels"`
	PixelDiffPercent float32 `json:"pixelDiffPercent"`
	MaxRGBDiffs      []int   `json:"maxRGBDiffs"`
	DiffImgUrl       string  `json:"diffImgUrl"`
	PosDigest        string  `json:"posDigest"`
}

// getTestDetails processes a tile and calculates the diff metrics for all
// untriaged digests.
func (a *Analyzer) getTestDetails(labeledTile *LabeledTile) GUITestDetails {
	glog.Infoln("Starting to extract test details.")
	result := map[string]*GUITestDetail{}
	totalTestCount := len(labeledTile.Traces)

	curTestCount := 0
	for testName, testTraces := range labeledTile.Traces {
		untriagedDigests := map[string]*GUIUntriagedDigest{}
		positiveDigests := map[string]*DigestInfo{}
		negativeDigests := map[string]*DigestInfo{}

		for _, oneTrace := range testTraces {
			for i, digest := range oneTrace.Digests {
				switch oneTrace.Labels[i] {
				case types.UNTRIAGED:
					// Capture the params for this digest. The diff metrics
					// are added once we have all positives for this test.
					if _, ok := untriagedDigests[digest]; !ok {
						untriagedDigests[digest] = &GUIUntriagedDigest{
							ParamsSet: make([]map[string]string, 0, len(testTraces)),
						}
					}
					untriagedDigests[digest].ParamsSet = append(untriagedDigests[digest].ParamsSet, oneTrace.Params)
				case types.POSITIVE:
					a.incDigestInfo(positiveDigests, digest)
				case types.NEGATIVE:
					a.incDigestInfo(negativeDigests, digest)
				}
			}
		}

		// get the positive digests as an array
		posDigestArr := make([]string, 0, len(positiveDigests))
		for posDigest, _ := range positiveDigests {
			posDigestArr = append(posDigestArr, posDigest)
		}

		// expand the info about the untriaged digests.
		for digest, _ := range untriagedDigests {
			dms := a.newGUIDiffMetrics(digest, posDigestArr)
			sort.Sort(dms)

			untriagedDigests[digest].ImgUrl = a.getUrl(digest)
			untriagedDigests[digest].Diffs = dms
		}

		result[testName] = &GUITestDetail{
			Untriaged: untriagedDigests,
			Positive:  positiveDigests,
			Negative:  negativeDigests,
		}

		curTestCount++
		glog.Infof("Processed %d/%d tests. (%f%%)", curTestCount, totalTestCount, float64(curTestCount)/float64(totalTestCount)*100.0)
	}

	glog.Infoln("Done extracting test details.")

	return result
}

func (a *Analyzer) incDigestInfo(digestMap map[string]*DigestInfo, digest string) {
	if _, ok := digestMap[digest]; !ok {
		digestMap[digest] = &DigestInfo{ImgUrl: a.getUrl(digest)}
	}
	digestMap[digest].Count++
}

func (a *Analyzer) getUrl(digest string) string {
	absPath, err := a.diffStore.AbsPath([]string{digest})
	if err != nil {
		glog.Errorf("Unable to resolve url for %s. Got error: %s", digest, err.Error())
		return ""
	}
	return a.pathToURLConverter(absPath[0])
}

func (a *Analyzer) newGUIDiffMetrics(digest string, posDigests []string) GUIDiffMetrics {
	result := GUIDiffMetrics(make([]*GUIDiffMetric, 0, len(posDigests)))

	dms, err := a.diffStore.Get(digest, posDigests)
	if err != nil {
		glog.Errorf("Unable to get diff for %s. Got error: %s", digest, err.Error())
		return nil
	}

	for i, dm := range dms {
		result = append(result, &GUIDiffMetric{
			NumDiffPixels:    dm.NumDiffPixels,
			PixelDiffPercent: dm.PixelDiffPercent,
			MaxRGBDiffs:      dm.MaxRGBDiffs,
			DiffImgUrl:       a.pathToURLConverter(dm.PixelDiffFilePath),
			PosDigest:        posDigests[i],
		})
	}
	return result
}
