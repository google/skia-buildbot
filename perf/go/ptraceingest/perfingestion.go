package ptraceingest

import (
	"net/http"

	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/cid"
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
	commitID, err := cid.FromHash(p.vcs, benchData.Hash)
	if err != nil {
		return err
	}

	return p.store.Add(commitID, getValueMap(benchData), resultsFile.Name())
}

// See ingestion.Processor interface.
func (p *perfProcessor) BatchFinished() error { return nil }
