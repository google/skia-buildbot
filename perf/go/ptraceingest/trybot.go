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
	patchset, err := strconv.ParseInt(benchData.PatchSet, 10, 64)
	if err != nil {
		return fmt.Errorf("Failed to parse trybot patch id: %s", err)
	}
	issueID, err := strconv.ParseInt(benchData.Issue, 10, 64)
	if err != nil {
		return fmt.Errorf("Failed to parse trybot issue id: %s", err)
	}

	issue, err := p.review.GetIssueProperties(issueID, false)
	if err != nil {
		return fmt.Errorf("Failed to get issue details %d: %s", issueID, err)
	}
	// Look through the Patchsets and find a matching one.
	var offset int = -1
	for i, pid := range issue.Patchsets {
		if pid == patchset {
			offset = i
			break
		}
	}
	if offset == -1 {
		return fmt.Errorf("Failed to find patchset %d in review %d", patchset, issueID)
	}

	source := fmt.Sprintf("%s/%s", CODE_REVIEW_URL, benchData.Issue)
	cid := &ptracestore.CommitID{
		Offset: offset,
		Source: source,
	}

	return p.store.Add(cid, getValueMap(benchData), resultsFile.Name())
}

// See ingestion.Processor interface.
func (p *perfTrybotProcessor) BatchFinished() error {
	return nil
}
