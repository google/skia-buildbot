package ptraceingest

import (
	"context"
	"net/http"
	"time"

	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/util"
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
func newPerfProcessor(vcs vcsinfo.VCS, config *sharedconfig.IngesterConfig, client *http.Client, eventBus eventbus.EventBus) (ingestion.Processor, error) {
	return &perfProcessor{
		store: ptracestore.Default,
		vcs:   vcs,
	}, nil
}

// See ingestion.Processor interface.
func (p *perfProcessor) Process(ctx context.Context, resultsFile ingestion.ResultFileLocation) error {
	r, err := resultsFile.Open()
	if err != nil {
		return err
	}
	defer util.Close(r)
	benchData, err := ingestcommon.ParseBenchDataFromReader(r)
	if err != nil {
		return err
	}
	commitID, err := cid.FromHash(ctx, p.vcs, benchData.Hash)
	if err != nil {
		return err
	}

	// The timestamp passed to Add() is ignored in a ptracestore backed store, and the
	// new Perf event based ingester doesn't use ingestion.Processor.
	now := time.Now()

	return p.store.Add(commitID, getValueMap(benchData), resultsFile.Name(), now)
}
