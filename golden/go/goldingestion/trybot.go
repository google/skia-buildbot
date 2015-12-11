package goldingestion

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/rietveld"
	"go.skia.org/infra/go/sharedconfig"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/config"
)

const (
	CONFIG_CODE_REVIEW_URL   = "CodeReviewURL"
	TIMESTAMP_LRU_CACHE_SIZE = 1000
)

func init() {
	ingestion.Register(config.CONSTRUCTOR_GOLD_TRYBOT, newGoldTrybotProcessor)
}

type goldTrybotProcessor struct {
	*goldProcessor
	review *rietveld.Rietveld

	//    map[issue:patchset] -> timestamp.
	cache map[string]time.Time
}

func newGoldTrybotProcessor(vcs vcsinfo.VCS, config *sharedconfig.IngesterConfig, client *http.Client) (ingestion.Processor, error) {
	processor, err := newGoldProcessor(vcs, config, client)
	if err != nil {
		return nil, err
	}

	// Get the underlying goldProcessor.
	gProcessor := processor.(*goldProcessor)
	ret := &goldTrybotProcessor{
		goldProcessor: gProcessor,
		review:        rietveld.New(config.ExtraParams[CONFIG_CODE_REVIEW_URL], nil),
		cache:         map[string]time.Time{},
	}

	// Change the function to extract the commitID.
	gProcessor.extractID = ret.getCommitID
	return ret, nil
}

// getCommitID overrides the function with the same name in goldProcessor.
func (g *goldTrybotProcessor) getCommitID(commit *vcsinfo.LongCommit, dmResults *DMResults) (*tracedb.CommitID, error) {
	var ts time.Time
	var ok bool
	var cacheId = fmt.Sprintf("%d:%d", dmResults.Issue, dmResults.Patchset)
	if ts, ok = g.cache[cacheId]; !ok {
		patchinfo, err := g.review.GetPatchset(dmResults.Issue, dmResults.Patchset)
		if err != nil {
			return nil, fmt.Errorf("Failed to retrieve trybot patch info: %s", err)
		}
		ts = patchinfo.Created

		// p.cache is a very crude LRU cache.
		if len(g.cache) > TIMESTAMP_LRU_CACHE_SIZE {
			g.cache = map[string]time.Time{}
		}
		g.cache[cacheId] = ts
	}

	source := fmt.Sprintf("%s%s/%d", string(TRYBOT_SRC), g.review.Url(), dmResults.Issue)
	return &tracedb.CommitID{
		Timestamp: ts.Unix(),
		ID:        strconv.FormatInt(dmResults.Patchset, 10),
		Source:    source,
	}, nil
}
