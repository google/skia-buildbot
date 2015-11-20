package perfingestion

import (
	"fmt"
	"strconv"
	"time"

	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/rietveld"
	"go.skia.org/infra/go/sharedconfig"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/types"
)

const (
	TIMESTAMP_LRU_CACHE_SIZE = 1000

	CODE_REVIEW_URL = "https://codereview.chromium.org"
)

// Register the processor with the ingestion framework.
func init() {
	ingestion.Register(config.CONSTRUCTOR_NANO_TRYBOT, newPerfTrybotProcessor)
}

// perfTrybotProcessor implements the ingestion.Processor interface for perf.
type perfTrybotProcessor struct {
	traceDB tracedb.DB
	review  *rietveld.Rietveld

	//    map[issue:patchset] -> timestamp.
	cache map[string]time.Time
}

// newPerfTrybotProcessor implements the ingestion.Constructor signature.
func newPerfTrybotProcessor(vcs vcsinfo.VCS, config *sharedconfig.IngesterConfig) (ingestion.Processor, error) {
	traceDB, err := tracedb.NewTraceServiceDBFromAddress(config.ExtraParams[CONFIG_TRACESERVICE], types.PerfTraceBuilder)
	if err != nil {
		return nil, err
	}

	return &perfTrybotProcessor{
		traceDB: traceDB,
		review:  rietveld.New(CODE_REVIEW_URL, nil),
		cache:   map[string]time.Time{},
	}, nil
}

// See ingestion.Processor interface.
func (p *perfTrybotProcessor) Process(resultsFile ingestion.ResultFileLocation) error {
	r, err := resultsFile.Open()
	if err != nil {
		return err
	}
	benchData, err := parseBenchDataFromReader(r)
	if err != nil {
		return err
	}
	issue, err := strconv.Atoi(benchData.Issue)
	if err != nil {
		return fmt.Errorf("Failed to parse trybot issue id: %s", err)
	}
	patchset, err := strconv.Atoi(benchData.PatchSet)
	if err != nil {
		return fmt.Errorf("Failed to parse trybot patch id: %s", err)
	}
	var ts time.Time
	var ok bool
	var cacheId = benchData.Issue + ":" + benchData.PatchSet
	if ts, ok = p.cache[cacheId]; !ok {
		patchinfo, err := p.review.GetPatchset(int64(issue), int64(patchset))
		if err != nil {
			return fmt.Errorf("Failed to retrieve trybot patch info: %s", err)
		}
		ts = patchinfo.Created
		// p.cache is a very crude LRU cache.
		if len(p.cache) > TIMESTAMP_LRU_CACHE_SIZE {
			p.cache = map[string]time.Time{}
		}
		p.cache[cacheId] = ts
	}
	source := fmt.Sprintf("%s/%d", CODE_REVIEW_URL, issue)

	cid := &tracedb.CommitID{
		Timestamp: ts,
		ID:        benchData.PatchSet,
		Source:    source,
	}

	// Add the column to the trace db.
	return p.traceDB.Add(cid, benchData.getTraceDBEntries())
}

// See ingestion.Processor interface.
func (p *perfTrybotProcessor) BatchFinished() error { return nil }
