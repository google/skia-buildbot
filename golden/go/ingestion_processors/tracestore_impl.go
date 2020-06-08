package ingestion_processors

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/ingestion"
	"go.skia.org/infra/golden/go/jsonio"
	"go.skia.org/infra/golden/go/shared"
	"go.skia.org/infra/golden/go/tracestore"
	"go.skia.org/infra/golden/go/tracestore/bt_tracestore"
	"go.skia.org/infra/golden/go/types"
)

const (
	// Configuration option that identifies a tracestore backed by BigTable.
	btGoldIngester = "gold-bt"

	btProjectConfig  = "BTProjectID"
	btInstanceConfig = "BTInstance"
	btTableConfig    = "BTTable"
)

// Register the processor with the ingestion framework.
func init() {
	ingestion.Register(btGoldIngester, newBTTraceStoreProcessor)
}

// newTraceStoreProcessor implements the ingestion.Constructor signature and creates
// a Processor that uses a BigTable-backed tracestore.
func newBTTraceStoreProcessor(ctx context.Context, vcs vcsinfo.VCS, config *sharedconfig.IngesterConfig, _ *http.Client) (ingestion.Processor, error) {
	btc := bt_tracestore.BTConfig{
		ProjectID:  config.ExtraParams[btProjectConfig],
		InstanceID: config.ExtraParams[btInstanceConfig],
		TableID:    config.ExtraParams[btTableConfig],
		VCS:        vcs,
	}

	bts, err := bt_tracestore.New(ctx, btc, true)
	if err != nil {
		return nil, skerr.Fmt("could not instantiate BT tracestore: %s", err)
	}
	return &btProcessor{
		ts:  bts,
		vcs: btc.VCS,
	}, nil
}

// btProcessor implements the ingestion.Processor interface for gold using
// the BigTable TraceStore
type btProcessor struct {
	ts  tracestore.TraceStore
	vcs vcsinfo.VCS
}

// Process implements the ingestion.Processor interface.
func (b *btProcessor) Process(ctx context.Context, resultsFile ingestion.ResultFileLocation) error {
	defer metrics2.FuncTimer().Stop()
	gr, err := processGoldResults(ctx, resultsFile)
	if err != nil {
		return skerr.Wrapf(err, "could not process results file")
	}

	if len(gr.Results) == 0 {
		sklog.Infof("ignoring file %s because it has no results", resultsFile.Name())
		return ingestion.IgnoreResultsFileErr
	}

	// If the target commit is not in the primary repository we look it up
	// in the secondary that has the primary as a dependency.
	targetHash, err := getCanonicalCommitHash(ctx, b.vcs, gr.GitHash)
	if err != nil {
		if err == ingestion.IgnoreResultsFileErr {
			return ingestion.IgnoreResultsFileErr
		}
		return skerr.Wrapf(err, "could not identify canonical commit from %q", gr.GitHash)
	}

	if ok, err := b.isOnMaster(ctx, targetHash); err != nil {
		return skerr.Wrapf(err, "could not determine branch for %s", targetHash)
	} else if !ok {
		sklog.Warningf("Commit %s is not in master branch", targetHash)
		return ingestion.IgnoreResultsFileErr
	}

	// Get the entries that should be added to the tracestore.
	entries, err := extractTraceStoreEntries(gr, resultsFile.Name())
	if err != nil {
		return skerr.Wrapf(err, "could not create entries")
	}

	t := time.Unix(resultsFile.TimeStamp(), 0)

	defer shared.NewMetricsTimer("put_tracestore_entry").Stop()

	sklog.Debugf("tracestore.Put(%s, %d entries, %s)", targetHash, len(entries), t)
	// Write the result to the tracestore.
	err = b.ts.Put(ctx, targetHash, entries, t)
	if err != nil {
		return skerr.Wrapf(err, "could not add entries to tracestore")
	}
	return nil
}

// isOnMaster returns true if the given commit hash is on the master branch.
func (b *btProcessor) isOnMaster(ctx context.Context, hash string) (bool, error) {
	// BT_VCS is configured to only look at master, so if we just look up the index of the hash,
	// we will know if it is on the master branch.
	// We can ignore the error, because it would be a "commit not found" error.
	if i, _ := b.vcs.IndexOf(ctx, hash); i >= 0 {
		return true, nil
	}

	if err := b.vcs.Update(ctx, true /*=pull*/, false /*=all branches*/); err != nil {
		return false, skerr.Wrapf(err, "could not update VCS")
	}
	if i, _ := b.vcs.IndexOf(ctx, hash); i >= 0 {
		return true, nil
	}
	return false, nil
}

// BatchFinished implements the ingestion.Processor interface.
func (b *btProcessor) BatchFinished() error { return nil }

// extractTraceStoreEntries creates a slice of tracestore.Entry for the given
// file. It will omit any entries that should be ignored. It returns an
// error if there were no un-ignored entries in the file.
func extractTraceStoreEntries(gr *jsonio.GoldResults, name string) ([]*tracestore.Entry, error) {
	ret := make([]*tracestore.Entry, 0, len(gr.Results))
	for _, result := range gr.Results {
		params, options := paramsAndOptions(gr, result)
		if err := shouldIngest(params, options); err != nil {
			sklog.Infof("Not ingesting %s : %s", name, err)
			continue
		}

		ret = append(ret, &tracestore.Entry{
			Params:  params,
			Options: options,
			Digest:  result.Digest,
		})
	}

	// If all results were ignored then we return an error.
	if len(ret) == 0 {
		return nil, skerr.Fmt("no valid results in file")
	}

	return ret, nil
}

// paramsAndOptions creates the params and options maps from a given file and entry.
func paramsAndOptions(gr *jsonio.GoldResults, r *jsonio.Result) (map[string]string, map[string]string) {
	params := make(map[string]string, len(gr.Key)+len(r.Key))
	for k, v := range gr.Key {
		params[k] = v
	}
	for k, v := range r.Key {
		params[k] = v
	}
	return params, r.Options
}

// shouldIngest returns a descriptive error if we should ignore an entry
// with these params/options.
func shouldIngest(params, options map[string]string) error {
	// Ignore anything that is not a png. In the early days (pre-2015), ext was omitted
	// but implied to be "png". Thus if ext is not provided, it will be ingested.
	// New entries (created by goldctl) will always have ext set.
	if ext, ok := options["ext"]; ok && (ext != "png") {
		return errors.New("ignoring non-png entry")
	}

	// Make sure the test name meets basic requirements.
	testName := params[types.PrimaryKeyField]

	// Ignore results that don't have a test given and log an error since that
	// should not happen. But we want to keep other results in the same input file.
	if testName == "" {
		return errors.New("missing test name")
	}

	// Make sure the test name does not exceed the allowed length.
	if len(testName) > types.MaximumNameLength {
		return fmt.Errorf("Received test name which is longer than the allowed %d bytes: %s", types.MaximumNameLength, testName)
	}

	return nil
}
