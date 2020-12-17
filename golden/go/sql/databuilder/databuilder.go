// package databuilder provides a tool for generating test data in a way that is easy for
// a human to update and understand.
package databuilder

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"strings"
	"time"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/types"
)

// SQLDataBuilder has methods on it for generating trace data and other related data in a way
// that can be easily turned into SQL table rows.
type SQLDataBuilder struct {
	commitBuilder   *CommitBuilder
	groupingKeys    []string
	symbolsToDigest map[rune]schema.Digest
	traceBuilders   []*TraceBuilder
}

// Commits returns a new CommitBuilder linked to this builder which will have a set. It panics if
// called more than once.
func (b *SQLDataBuilder) Commits() *CommitBuilder {
	if b.commitBuilder != nil {
		sklog.Fatalf("Cannot call Commits() more than once.")
	}
	b.commitBuilder = &CommitBuilder{}
	return b.commitBuilder
}

// UseDigests loads a mapping of symbols (runes) to the digest that they represent. This allows
// specifying the trace history be done with a string of characters. If a rune is invalid or
// the digests are invalid, this will panic. It panics if called more than once.
func (b *SQLDataBuilder) UseDigests(symbolsToDigest map[rune]types.Digest) {
	if b.symbolsToDigest != nil {
		sklog.Fatalf("Cannot call UseDigests() more than once.")
	}
	m := make(map[rune]schema.Digest, len(symbolsToDigest))
	for symbol, digest := range symbolsToDigest {
		if symbol == '-' {
			sklog.Fatalf("Cannot map something to -")
		}
		d, err := sql.DigestToBytes(digest)
		if err != nil {
			sklog.Fatalf("Invalid digest %q: %s", digest, err)
		}
		m[symbol] = d
	}
	b.symbolsToDigest = m
}

// SetGroupingKeys specifies which keys from a Trace's params will be used to define the grouping.
// It panics if called more than once.
func (b *SQLDataBuilder) SetGroupingKeys(fields ...string) {
	if b.groupingKeys != nil {
		sklog.Fatalf("Cannot call SetGroupingKeys() more than once.")
	}
	b.groupingKeys = fields
}

// TracesWithCommonKeys returns a new TraceBuilder for building a set of related traces. This can
// be called more than once - all data will be combined at the end. It panics if any of its
// prerequisites have not been called.
func (b *SQLDataBuilder) TracesWithCommonKeys(params paramtools.Params) *TraceBuilder {
	if len(b.commitBuilder.commits) == 0 {
		sklog.Fatalf("Must add commits before traces")
	}
	if len(b.groupingKeys) == 0 {
		sklog.Fatalf("Must add grouping keys before traces")
	}
	if len(b.symbolsToDigest) == 0 {
		sklog.Fatalf("Must add digests before traces")
	}
	tb := &TraceBuilder{
		commits:         b.commitBuilder.commits,
		commonKeys:      params,
		symbolsToDigest: b.symbolsToDigest,
		groupingKeys:    b.groupingKeys,
	}
	b.traceBuilders = append(b.traceBuilders, tb)
	return tb
}

// GenerateStructs should be called when all the data has been loaded in for a given setup and
// it will generate the SQL rows as represented in a schema.Tables. If any validation steps fail,
// it will panic.
func (b *SQLDataBuilder) GenerateStructs() schema.Tables {
	var rv schema.Tables
	commitsWithData := map[schema.CommitID]bool{}
	for _, builder := range b.traceBuilders {
		// Add unique rows from the trace builders.
	nextOption:
		for _, opt := range builder.options {
			for _, existingOpt := range rv.Options {
				if bytes.Equal(opt.OptionsID, existingOpt.OptionsID) {
					continue nextOption
				}
			}
			rv.Options = append(rv.Options, opt)
		}
	nextGrouping:
		for _, g := range builder.groupings {
			for _, existingG := range rv.Groupings {
				if bytes.Equal(g.GroupingID, existingG.GroupingID) {
					continue nextGrouping
				}
			}
			rv.Groupings = append(rv.Groupings, g)
		}
	nextSource:
		for _, sf := range builder.sourceFiles {
			for _, existingSF := range rv.SourceFiles {
				if bytes.Equal(sf.SourceFileID, existingSF.SourceFileID) {
					continue nextSource
				}
			}
			rv.SourceFiles = append(rv.SourceFiles, sf)
		}
		for _, t := range builder.traces {
			for _, existingT := range rv.Traces {
				if bytes.Equal(t.TraceID, existingT.TraceID) {
					// Having a duplicate trace means that there are duplicate TraceValues entries
					// and that is not intended.
					sklog.Fatalf("Duplicate trace found: %v", t.Keys)
				}
			}
			rv.Traces = append(rv.Traces, t)
		}
		for _, tv := range builder.traceValues {
			if tv != nil {
				rv.TraceValues = append(rv.TraceValues, *tv)
				commitsWithData[tv.CommitID] = true
			}
		}
	}
	rv.Commits = b.commitBuilder.commits
	for i := range rv.Commits {
		cid := rv.Commits[i].CommitID
		if commitsWithData[cid] {
			rv.Commits[i].HasData = true
		}
	}
	return rv
}

// CommitBuilder has methods for easily building commit history. All methods are chainable.
type CommitBuilder struct {
	commits    []schema.CommitRow
	previousID int
}

// Append adds a commit whose ID is one higher than the previous commits ID. It panics if
// the commitTime is not formatted to RFC3339.
func (b *CommitBuilder) Append(author, subject, commitTime string) *CommitBuilder {
	commitID := b.previousID + 1
	gitHash := fmt.Sprintf("%04d", commitID)
	// A true githash is 40 hex characters, so we repeat the 4 digits of the commitID 10 times.
	gitHash = strings.Repeat(gitHash, 10)
	ct, err := time.Parse(time.RFC3339, commitTime)
	if err != nil {
		sklog.Fatalf("Invalid time %q: %s", commitTime, err)
	}
	b.commits = append(b.commits, schema.CommitRow{
		CommitID:   schema.CommitID(commitID),
		GitHash:    gitHash,
		CommitTime: ct,
		Author:     author,
		Subject:    subject,
		HasData:    false,
	})
	b.previousID = commitID
	return b
}

// TraceBuilder has methods for easily building trace data. All methods are chainable.
type TraceBuilder struct {
	// inputs needed upon creation
	commits         []schema.CommitRow
	commonKeys      paramtools.Params
	groupingKeys    []string
	symbolsToDigest map[rune]schema.Digest

	// built as a result of the calling methods
	groupings   []schema.GroupingRow
	numTraces   int
	options     []schema.OptionsRow
	sourceFiles []schema.SourceFileRow
	traceValues []*schema.TraceValueRow // a flattened 2d array of TraceValues
	traces      []schema.TraceRow
}

// History takes in a slice of strings, with each string representing the history of a trace. Each
// string must have a number of symbols equal to the length of the number of commits. A dash '-'
// means no data at that commit; any other symbol must match the previous call to UseDigests().
// If any data is invalid or missing, this method panics.
func (b *TraceBuilder) History(traceHistories []string) *TraceBuilder {
	if len(b.traceValues) > 0 {
		sklog.Fatalf("History must be called only once.")
	}
	b.numTraces = len(traceHistories)
	// traceValues will have length len(commits) * numTraces after this is complete. Some entries
	// may be nil to represent "no data" and will be stripped out later.
	for _, th := range traceHistories {
		if len(th) != len(b.commits) {
			sklog.Fatalf("history %q is of invalid length: expected %d", th, len(b.commits))
		}
		for i, symbol := range th {
			if symbol == '-' {
				b.traceValues = append(b.traceValues, nil)
				continue
			}
			digest, ok := b.symbolsToDigest[symbol]
			if !ok {
				sklog.Fatalf("Unknown symbol in trace history %s", symbol)
			}
			b.traceValues = append(b.traceValues, &schema.TraceValueRow{
				CommitID: b.commits[i].CommitID,
				Digest:   digest,
			})
		}
	}
	return b
}

// Keys specifies the params for each trace. It must be called after History() and the keys param
// must have the same number of elements that the call to History() had. The nth element here
// represents the nth trace history. This method panics if any trace would end up being identical
// or lacks the grouping data. This method panics if called with incorrect parameters or at the
// wrong time in building chain.
func (b *TraceBuilder) Keys(keys []paramtools.Params) *TraceBuilder {
	if b.numTraces == 0 {
		sklog.Fatalf("Keys must be called after history loaded")
	}
	if len(b.traces) > 0 {
		sklog.Fatalf("Keys must only once")
	}
	if len(keys) != b.numTraces {
		sklog.Fatalf("Expected one set of keys for each trace")
	}
	// We now have enough data to make all the traces.
	seenTraces := map[schema.SerializedJSON]bool{}
	for i, traceParams := range keys {
		traceParams.Add(b.commonKeys)
		grouping := make(map[string]string, len(keys))
		for _, gk := range b.groupingKeys {
			val, ok := traceParams[gk]
			if !ok {
				sklog.Fatalf("Missing grouping key %q from %v", gk, traceParams)
			}
			grouping[gk] = val
		}
		groupingJSON, groupingID := sql.SerializeMap(grouping)
		traceJSON, traceID := sql.SerializeMap(traceParams)
		if seenTraces[traceJSON] {
			sklog.Fatalf("Found identical trace %s", traceJSON)
		}
		numCommits := len(b.commits)
		for _, row := range b.traceValues[i*numCommits : (i+1)*numCommits] {
			if row != nil {
				row.GroupingID = groupingID
				row.TraceID = traceID
				row.Shard = sql.ComputeTraceValueShard(traceID)
			}
		}
		b.groupings = append(b.groupings, schema.GroupingRow{
			GroupingID: groupingID,
			Keys:       groupingJSON,
		})
		b.traces = append(b.traces, schema.TraceRow{
			TraceID:              traceID,
			Corpus:               traceParams[types.CorpusField],
			GroupingID:           groupingID,
			Keys:                 traceJSON,
			MatchesAnyIgnoreRule: schema.NotSet,
		})
	}
	return b
}

// OptionsAll applies the given options for all data points provided in history.
func (b *TraceBuilder) OptionsAll(opts paramtools.Params) *TraceBuilder {
	xopts := make([]paramtools.Params, b.numTraces)
	for i := range xopts {
		xopts[i] = opts
	}
	return b.OptionsPerTrace(xopts)
}

// OptionsPerTrace applies the given optional params to the traces created in History. The number
// of options is expected to match the number of traces. It panics if called more than once or
// at the wrong time.
func (b *TraceBuilder) OptionsPerTrace(xopts []paramtools.Params) *TraceBuilder {
	if b.numTraces == 0 {
		sklog.Fatalf("Options* must be called after history loaded")
	}
	if len(b.options) > 0 {
		sklog.Fatalf("Must call Options only once")
	}
	if len(xopts) != b.numTraces {
		sklog.Fatalf("Must have one options per trace")
	}
	for i, tv := range b.traceValues {
		if tv == nil {
			continue
		}
		optJSON, optionsID := sql.SerializeMap(xopts[i/len(b.commits)])
		tv.OptionsID = optionsID
		b.options = append(b.options, schema.OptionsRow{
			OptionsID: optionsID,
			Keys:      optJSON,
		})
	}
	return b
}

// IngestedFrom applies the given list of files and ingested times to the provided data.
// The number of filenames and ingestedDates is expected to match the number of commits; if no
// data is at that commit, it is ok to have both entries be empty stirng. It panics if any inputs
// are invalid.
func (b *TraceBuilder) IngestedFrom(filenames, ingestedDates []string) *TraceBuilder {
	if len(b.traceValues) == 0 {
		sklog.Fatalf("IngestedFrom must be called after history")
	}
	if len(filenames) != len(b.commits) {
		sklog.Fatalf("Expected %d files", len(b.commits))
	}
	if len(ingestedDates) != len(b.commits) {
		sklog.Fatalf("Expected %d dates", len(b.commits))
	}
	for i, tv := range b.traceValues {
		if tv == nil {
			continue
		}
		name := filenames[i%len(b.commits)]
		if name == "" {
			sklog.Fatalf("filename cannot be empty if used in a trace")
		}
		h := md5.Sum([]byte(name))
		tv.SourceFileID = h[:]
	}

	for i := range filenames {
		name, ingestedDate := filenames[i], ingestedDates[i]
		if name == "" && ingestedDate == "" {
			continue // not used by any traces
		}
		if name == "" || ingestedDate == "" {
			sklog.Fatalf("both name and date should be empty, if one is")
		}
		h := md5.Sum([]byte(name))
		sourceID := h[:]

		d, err := time.Parse(time.RFC3339, ingestedDate)
		if err != nil {
			sklog.Fatalf("Invalid date format %q: %s", ingestedDate, err)
		}
		b.sourceFiles = append(b.sourceFiles, schema.SourceFileRow{
			SourceFileID: sourceID,
			SourceFile:   name,
			LastIngested: d,
		})
	}
	return b
}
