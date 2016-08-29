package ptraceingest

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/rietveld"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/ingestcommon"
	"go.skia.org/infra/perf/go/ptracestore"
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
		review: rietveld.New(CODE_REVIEW_URL, client),
	}, nil
}

// See ingestion.Processor interface.
func (p *perfTrybotProcessor) Process(resultsFile ingestion.ResultFileLocation) error {
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
		glog.Infof("Ignoring Gerrit issue %s/%s for now.", benchData.Issue, benchData.PatchSet)
		return ingestion.IgnoreResultsFileErr
	}
	patchset, err := strconv.Atoi(benchData.PatchSet)
	if err != nil {
		return fmt.Errorf("Failed to parse trybot patch id: %s", err)
	}
	source := fmt.Sprintf("%s/%s", CODE_REVIEW_URL, benchData.Issue)
	cid := &ptracestore.CommitID{
		Offset: patchset,
		Source: source,
	}

	return p.store.Add(cid, getValueMap(benchData), resultsFile.Name())
}

// See ingestion.Processor interface.
func (p *perfTrybotProcessor) BatchFinished() error {
	return nil
}
