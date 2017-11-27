package ptraceingest

import (
	"context"
	"net/http"

	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/rietveld"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/ingestcommon"
	"go.skia.org/infra/perf/go/ptracestore"
)

const (
	TIMESTAMP_LRU_CACHE_SIZE = 1000
)

// Register the processor with the ingestion framework.
func init() {
	ingestion.Register(config.CONSTRUCTOR_NANO_TRYBOT, newPerfTrybotProcessor)
}

// perfTrybotProcessor implements the ingestion.Processor interface for perf.
//
// Note that ptracestore.Init() needs to be called before starting ingestion so
// that ptracestore.Default is set correctly.
type perfTrybotProcessor struct {
	store  ptracestore.PTraceStore
	review *rietveld.Rietveld
}

// newPerfTrybotProcessor implements the ingestion.Constructor signature.
func newPerfTrybotProcessor(vcs vcsinfo.VCS, config *sharedconfig.IngesterConfig, client *http.Client) (ingestion.Processor, error) {
	return &perfTrybotProcessor{
		store:  ptracestore.Default,
		review: rietveld.New(cid.CODE_REVIEW_URL, client),
	}, nil
}

// See ingestion.Processor interface.
func (p *perfTrybotProcessor) Process(ctx context.Context, resultsFile ingestion.ResultFileLocation) error {
	r, err := resultsFile.Open()
	if err != nil {
		return err
	}
	benchData, err := ingestcommon.ParseBenchDataFromReader(r)
	if err != nil {
		return err
	}

	// Ignore results from Gerrit for now.
	if benchData.IsGerritIssue() {
		sklog.Infof("Ignoring Gerrit issue %s/%s for now.", benchData.Issue, benchData.PatchSet)
		return ingestion.IgnoreResultsFileErr
	}

	commitID, err := cid.FromIssue(p.review, benchData.Issue, benchData.PatchSet)
	if err != nil {
		return err
	}

	return p.store.Add(commitID, getValueMap(benchData), resultsFile.Name())
}

// See ingestion.Processor interface.
func (p *perfTrybotProcessor) BatchFinished() error {
	return nil
}
