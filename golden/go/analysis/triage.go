package analysis

import (
	"sort"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/golden/go/types"
	ptypes "go.skia.org/infra/perf/go/types"
)

// GUITestDetails is an output type with triage information grouped by tests.
// TODO(stephana): Factor commits handling out of triage and into into a separate REST endpoint.
type GUITestDetails struct {
	Commits         []*ptypes.Commit                `json:"commits"`
	CommitsByDigest map[string]map[string][]int     `json:"commitsByDigest"`
	AllParams       map[string][]string             `json:"allParams"`
	Tests           []*GUITestDetail                `json:"tests"`
	Query           map[string][]string             `json:"query"`
	Blames          map[string][]*BlameDistribution `json:"blames"`
	testsMap        map[string]int
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
	Diameter  int                            `json:"diameter"` // Max distance between any two images.
}

// DigestInfo contains the image URL and the occurence count of a digest.
type DigestInfo struct {
	CommitIds   []int                     `json:"commitIds"`
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

// GUITestDetailSortable is a wrapper to sort a slice of GUITestDetail.
type GUITestDetailSortable []*GUITestDetail

func (g GUITestDetailSortable) Len() int           { return len(g) }
func (g GUITestDetailSortable) Swap(i, j int)      { g[i], g[j] = g[j], g[i] }
func (g GUITestDetailSortable) Less(i, j int) bool { return g[i].Name < g[j].Name }

// getTestDetails processes a tile and calculates the diff metrics for all
// untriaged digests.
func (a *Analyzer) getTestDetails(state *AnalyzeState) *GUITestDetails {
	glog.Infof("Latest commit: %v", state.Tile.Commits[len(state.Tile.Commits)-1])
	glog.Infoln("Starting to extract test details.")
	nTests := len(state.Tile.Traces)
	resultCh := make(chan *GUITestDetail, nTests)

	for testName, testTraces := range state.Tile.Traces {
		go a.processOneTestDetail(testName, testTraces, resultCh)
	}

	// Wait for the results to finish.
	result := make([]*GUITestDetail, 0, nTests)
	for i := 0; i < nTests; i++ {
		result = append(result, <-resultCh)
		glog.Infof("Processed %d/%d tests. (%f%%)", len(result), nTests, float64(len(result))/float64(nTests)*100.0)
	}

	// Sort the resulting tests by name.
	sort.Sort(GUITestDetailSortable(result))

	// Build the test lookup map.
	testsMap := make(map[string]int, nTests)
	for idx, t := range result {
		testsMap[t.Name] = idx
	}

	glog.Infoln("Done extracting test details.")

	return &GUITestDetails{
		Commits:         state.Tile.Commits,
		CommitsByDigest: state.Tile.CommitsByDigest,
		AllParams:       state.Index.getAllParams(nil),
		Tests:           result,
		testsMap:        testsMap,
	}
}

func (a *Analyzer) updateTestDetails(labeledTestDigests map[string]types.TestClassification, state *AnalyzeState) {
	glog.Infof("Latest commit: %v", state.TestDetails.Commits[len(state.TestDetails.Commits)-1])
	glog.Infoln("Starting to update test details.")
	nTests := len(labeledTestDigests)
	resultCh := make(chan *GUITestDetail, nTests)

	for testName := range labeledTestDigests {
		go a.processOneTestDetail(testName, state.Tile.Traces[testName], resultCh)
	}

	// Wait for the results to finish.
	curr := state.TestDetails.Tests
	for i := 0; i < nTests; i++ {
		result := <-resultCh

		// find the result in the current tile and replace it.
		idx := sort.Search(len(curr), func(j int) bool { return curr[j].Name >= result.Name })
		// We found the entry.
		if (idx < len(curr)) && (curr[idx].Name == result.Name) {
			curr[idx] = result
		}
	}

	glog.Infoln("Done updating test details.")
}

func (a *Analyzer) processOneTestDetail(testName string, testTraces []*LabeledTrace, resultCh chan<- *GUITestDetail) {
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

	resultCh <- &GUITestDetail{
		Name:      testName,
		Untriaged: untriagedDigests,
		Positive:  positiveDigests,
		Negative:  negativeDigests,
		Diameter:  a.diameter(testTraces),
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
	absPath := a.storages.DiffStore.AbsPath([]string{digest})
	return a.pathToURLConverter(absPath[digest])
}

// diameter returns an approximate diameter of the images in a test.
//
// The value returned is only an approximation. It works by taking all the
// positive and untriaged digests and sorts them, so comparisons are stable. It
// then does pair-wise comparisons between digest N and N+1.  The idea is that
// all the positives should be close together, so a bad image will be "far"
// from all the good images, so it doesn't matter which good image you compare
// it to.
func (a *Analyzer) diameter(testTraces []*LabeledTrace) int {
	max := 0
	digestMap := map[string]bool{}

	for _, oneTrace := range testTraces {
		for i, digest := range oneTrace.Digests {
			switch oneTrace.Labels[i] {
			case types.UNTRIAGED:
				digestMap[digest] = true
			case types.POSITIVE:
				digestMap[digest] = true
			}
		}
	}
	digests := []string{}
	for k, _ := range digestMap {
		digests = append(digests, k)
	}
	sort.Strings(digests)

	for {
		if len(digests) <= 2 {
			break
		}
		dms, err := a.storages.DiffStore.Get(digests[0], digests[1:2])
		digests = digests[1:]
		if err != nil {
			glog.Errorf("Unable to get diff: %s", err)
			continue
		}
		for _, dm := range dms {
			if dm.NumDiffPixels > max {
				max = dm.NumDiffPixels
			}
		}
	}
	return max
}

func (a *Analyzer) newGUIDiffMetrics(digest string, posDigests []string) GUIDiffMetrics {
	result := GUIDiffMetrics(make([]*GUIDiffMetric, 0, len(posDigests)))

	dms, err := a.storages.DiffStore.Get(digest, posDigests)
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
