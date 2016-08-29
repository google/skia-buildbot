package ptraceingest

import (
	"fmt"
	"net/http"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/ingestcommon"
	"go.skia.org/infra/perf/go/ptracestore"
)

// Register the processor with the ingestion framework.
func init() {
	ingestion.Register(config.CONSTRUCTOR_NANO, newPerfProcessor)
}

// perfProcessor implements the ingestion.Processor interface for perf.
type perfProcessor struct {
	store ptracestore.PTraceStore
	vcs   vcsinfo.VCS
}

// newPerfProcessor implements the ingestion.Constructor signature.
//
// Note that ptracestore.Init() needs to be called before starting ingestion so
// that ptracestore.Default is set correctly.
func newPerfProcessor(vcs vcsinfo.VCS, config *sharedconfig.IngesterConfig, client *http.Client) (ingestion.Processor, error) {
	return &perfProcessor{
		store: ptracestore.Default,
		vcs:   vcs,
	}, nil
}

// See ingestion.Processor interface.
func (p *perfProcessor) Process(resultsFile ingestion.ResultFileLocation) error {
	r, err := resultsFile.Open()
	if err != nil {
		return err
	}
	benchData, err := ingestcommon.ParseBenchDataFromReader(r)
	if err != nil {
		return err
	}
	commit, err := p.vcs.Details(benchData.Hash, true)
	if err != nil {
		return err
	}
	if !commit.Branches["master"] {
		glog.Warningf("Commit %s is not in master branch.", commit.Hash)
		return ingestion.IgnoreResultsFileErr
	}
	offset, err := p.vcs.IndexOf(commit.Hash)
	if err != nil {
		return fmt.Errorf("Could not ingest, hash not found %q: %s", commit.Hash, err)
	}
	cid := &ptracestore.CommitID{
		Offset: offset,
		Source: "master",
	}

	return p.store.Add(cid, getValueMap(benchData), resultsFile.Name())
}

// See ingestion.Processor interface.
func (p *perfProcessor) BatchFinished() error { return nil }
