package analysis

import (
	"sort"

	"github.com/golang/glog"

	"skia.googlesource.com/buildbot.git/golden/go/types"
	ptypes "skia.googlesource.com/buildbot.git/perf/go/types"
)

// GUITestDetails is an output type with triage information grouped by tests.
type GUITestDetails struct {
	Commits   []*ptypes.Commit    `json:"commits"`
	AllParams map[string][]string `json:"allParams"`
	Tests     []*GUITestDetail    `json:"tests"`
	Query     map[string][]string `json:"query"`
	testsMap  map[string]int
}

func (g *GUITestDetails) lookup(testName string) *GUITestDetail {
	if idx, ok := g.testsMap[testName]; ok {
		return g.Tests[idx]
	}
	return nil
}

// GUITestDetail contains the untriaged, positive and negative digests of
// a test with all the information necessary to triage the digests.
type GUITestDetail struct {
	Name      string                         `json:"name"`
	Untriaged map[string]*GUIUntriagedDigest `json:"untriaged"`
	Positive  map[string]*DigestInfo         `json:"positive"`
	Negative  map[string]*DigestInfo         `json:"negative"`
}

// DigestInfo contains the image URL and the occurence count of a digest.
type DigestInfo struct {
	ImgUrl      string                    `json:"imgUrl"`
	Count       int                       `json:"count"`
	ParamCounts map[string]map[string]int `json:"paramCounts"`
}

// GUIUntriagedDigest is an output type for a single digest to be triaged.
// Aside from the digests image url and params it also contains metrics
// comparing it to the positive digests.
type GUIUntriagedDigest struct {
	// This is also an instance of DigestInfo
	DigestInfo
	Diffs GUIDiffMetrics `json:"diffs"`
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
	MaxRGBADiffs     []int   `json:"maxRGBADiffs"`
	DiffImgUrl       string  `json:"diffImgUrl"`
	PosDigest        string  `json:"posDigest"`
}

// getTestDetails processes a tile and calculates the diff metrics for all
// untriaged digests.
func (a *Analyzer) getTestDetails(labeledTile *LabeledTile) *GUITestDetails {
	glog.Infoln("Starting to extract test details.")
	result := []*GUITestDetail{}
	testsMap := map[string]int{}
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
					if _, ok := untriagedDigests[digest]; !ok {
						untriagedDigests[digest] = &GUIUntriagedDigest{
							DigestInfo: DigestInfo{
								ImgUrl:      a.getUrl(digest),
								ParamCounts: map[string]map[string]int{},
							},
						}
					}
					a.incDigestInfo(&untriagedDigests[digest].DigestInfo, digest, oneTrace.Params)
				case types.POSITIVE:
					positiveDigests[digest] = a.incDigestInfo(positiveDigests[digest], digest, oneTrace.Params)
				case types.NEGATIVE:
					negativeDigests[digest] = a.incDigestInfo(negativeDigests[digest], digest, oneTrace.Params)
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

		result = append(result, &GUITestDetail{
			Name:      testName,
			Untriaged: untriagedDigests,
			Positive:  positiveDigests,
			Negative:  negativeDigests,
		})
		testsMap[testName] = len(result) - 1

		curTestCount++
		glog.Infof("Processed %d/%d tests. (%f%%)", curTestCount, totalTestCount, float64(curTestCount)/float64(totalTestCount)*100.0)
	}

	glog.Infoln("Done extracting test details.")

	return &GUITestDetails{
		Commits:   labeledTile.Commits,
		AllParams: labeledTile.allParams,
		Tests:     result,
		testsMap:  testsMap,
	}
}

func (a *Analyzer) incDigestInfo(digestInfo *DigestInfo, digest string, params map[string]string) *DigestInfo {
	if digestInfo == nil {
		digestInfo = &DigestInfo{
			ImgUrl:      a.getUrl(digest),
			ParamCounts: map[string]map[string]int{},
		}
	}
	digestInfo.Count++
	incParamCounts(digestInfo.ParamCounts, params)
	return digestInfo
}

func incParamCounts(paramCounts map[string]map[string]int, params map[string]string) {
	for k, v := range params {
		if _, ok := paramCounts[k]; !ok {
			paramCounts[k] = map[string]int{}
		}
		paramCounts[k][v]++
	}
}

func (a *Analyzer) getUrl(digest string) string {
	absPath := a.diffStore.AbsPath([]string{digest})
	return a.pathToURLConverter(absPath[digest])
}

func (a *Analyzer) newGUIDiffMetrics(digest string, posDigests []string) GUIDiffMetrics {
	result := GUIDiffMetrics(make([]*GUIDiffMetric, 0, len(posDigests)))

	dms, err := a.diffStore.Get(digest, posDigests)
	if err != nil {
		glog.Errorf("Unable to get diff for %s. Got error: %s", digest, err)
		return nil
	}

	for posDigest, dm := range dms {
		result = append(result, &GUIDiffMetric{
			NumDiffPixels:    dm.NumDiffPixels,
			PixelDiffPercent: dm.PixelDiffPercent,
			MaxRGBADiffs:     dm.MaxRGBADiffs,
			DiffImgUrl:       a.pathToURLConverter(dm.PixelDiffFilePath),
			PosDigest:        posDigest,
		})
	}
	return result
}
