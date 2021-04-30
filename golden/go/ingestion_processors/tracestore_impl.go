package ingestion_processors

import (
	"context"
	"errors"
	"io/ioutil"
	"regexp"
	"time"

	"go.opencensus.io/trace"

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
	// BigTableTraceStore identifies a primary-branch ingester backed by BigTable.
	BigTableTraceStore = "big_table_tracestore"

	btProjectConfig  = "BTProjectID"
	btInstanceConfig = "BTInstance"
	btTableConfig    = "BTTable"
)

// PrimaryBranchBigTable creates a Processor that uses a BigTable-backed tracestore.
func PrimaryBranchBigTable(ctx context.Context, src ingestion.Source, configParams map[string]string, vcs vcsinfo.VCS) (ingestion.Processor, error) {
	btc := bt_tracestore.BTConfig{
		ProjectID:  configParams[btProjectConfig],
		InstanceID: configParams[btInstanceConfig],
		TableID:    configParams[btTableConfig],
		VCS:        vcs,
	}

	bts, err := bt_tracestore.New(ctx, btc, true)
	if err != nil {
		return nil, skerr.Fmt("could not instantiate BT tracestore: %s", err)
	}
	return &btProcessor{
		ts:     bts,
		vcs:    btc.VCS,
		source: src,
	}, nil
}

// btProcessor implements the ingestion.Processor interface for gold using
// the BigTable TraceStore
type btProcessor struct {
	ts     tracestore.TraceStore
	vcs    vcsinfo.VCS
	source ingestion.Source
}

// HandlesFile returns true if the configured source handles this file.
func (b *btProcessor) HandlesFile(name string) bool {
	return b.source.HandlesFile(name)
}

// Process implements the ingestion.Processor interface.
func (b *btProcessor) Process(ctx context.Context, fileName string) error {
	ctx, span := trace.StartSpan(ctx, "ingestion_BigTableProcess")
	defer span.End()
	r, err := b.source.GetReader(ctx, fileName)
	if err != nil {
		return skerr.Wrap(err)
	}
	gr, err := processGoldResults(ctx, r)
	if err != nil {
		return skerr.Wrapf(err, "could not process file %s from source %s", fileName, b.source)
	}

	if len(gr.Results) == 0 {
		sklog.Infof("file %s had no results", fileName)
		return nil
	}
	span.AddAttributes(trace.Int64Attribute("num_results", int64(len(gr.Results))))

	// If the target commit is not in the primary repository we look it up
	// in the secondary that has the primary as a dependency.
	var targetHash string
	if gr.CommitID != "" && gr.CommitMetadata != "" {
		targetHash, err = b.lookupChromeOSMetadata(ctx, gr.CommitMetadata)
		if err != nil {
			return skerr.Wrapf(err, "Looking up githash from Chromeos metadata: %q", gr.CommitMetadata)
		}
	} else {
		targetHash, err = getCanonicalCommitHash(ctx, b.vcs, gr.GitHash)
		if err != nil {
			return skerr.Wrapf(err, "could not identify canonical commit from %q", gr.GitHash)
		}

		if ok, err := b.isOnMaster(ctx, targetHash); err != nil {
			return skerr.Wrapf(err, "could not determine branch for %s", targetHash)
		} else if !ok {
			return skerr.Fmt("Commit %s is not in primary branch", targetHash)
		}
	}

	// Get the entries that should be added to the tracestore.
	entries, err := extractTraceStoreEntries(gr, fileName)
	if err != nil {
		// Probably invalid, don't retry
		return skerr.Wrapf(err, "could not create entries")
	}

	defer shared.NewMetricsTimer("put_tracestore_entry").Stop()

	sklog.Debugf("Ingested %d entries to commit %s from file %s", len(entries), targetHash, fileName)
	// Write the result to the tracestore.
	err = b.ts.Put(ctx, targetHash, entries, time.Now())
	if err != nil {
		sklog.Errorf("Could not add entries to tracestore for file %s: %s", fileName, err)
		return ingestion.ErrRetryable
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
func paramsAndOptions(gr *jsonio.GoldResults, r jsonio.Result) (map[string]string, map[string]string) {
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
		return skerr.Fmt("received test name which is longer than the allowed %d bytes: %s", types.MaximumNameLength, testName)
	}

	return nil
}

// Example input is gs://chromeos-image-archive/sentry-release/R92-13944.0.0/manifest.xml
var metadataURLRegex = regexp.MustCompile(`gs://(?P<bucket>[a-z\-]+)/(?P<object>.+)`)

// Instead of parsing XML, we use a simple regex to extract the one field we need.
var revisionRegex = regexp.MustCompile(`name="chromiumos/platform/tast-tests" path="src/platform/tast-tests" revision="(?P<githash>[a-f0-9]+)"`)

// lookupChromeOSMetadata extracts the git hash (of the tast-tests repo) from the provided
// manifest.xml file. This manifest file contains the revisions of all ChromeOS repos at the
// time of the build, but we only are tracking the tast-tests one.
func (b *btProcessor) lookupChromeOSMetadata(ctx context.Context, gcsURL string) (string, error) {
	gcsClient := b.source.(*ingestion.GCSSource).Client
	matches := metadataURLRegex.FindStringSubmatch(gcsURL)
	if matches == nil {
		return "", skerr.Fmt("Invalid metadata string %q", gcsURL)
	}
	// match[1] is the bucket, match[2] is the object.
	r, err := gcsClient.Bucket(matches[1]).Object(matches[2]).NewReader(ctx)
	if err != nil {
		return "", skerr.Wrapf(err, "getting reader for %q", gcsURL)
	}
	xmlBytes, err := ioutil.ReadAll(r)
	if err != nil {
		return "", skerr.Wrapf(err, "reading from %q", gcsURL)
	}
	match := revisionRegex.FindStringSubmatch(string(xmlBytes))
	if match == nil {
		return "", skerr.Fmt("Could not find tast-tests revision in XML from %q", gcsURL)
	}
	return match[1], nil
}
