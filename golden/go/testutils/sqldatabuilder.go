package testutils

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"strings"
	"time"

	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/go/skerr"

	"go.skia.org/infra/golden/go/sql"

	"go.skia.org/infra/golden/go/types"

	"go.skia.org/infra/go/paramtools"

	"go.skia.org/infra/golden/go/sql/schema"
)

type SQLDataBuilder struct {
	commitBuilder   *CommitBuilder
	symbolsToDigest map[rune]schema.Digest
	traceBuilders   []*TraceBuilder
	groupingKeys    []string
}

func (b *SQLDataBuilder) DenseHistory() *CommitBuilder {
	b.commitBuilder = &CommitBuilder{}
	return b.commitBuilder
}

func (b *SQLDataBuilder) UseDigests(symbolsToDigest map[rune]types.Digest) {
	m := make(map[rune]schema.Digest, len(symbolsToDigest))
	for symbol, digest := range symbolsToDigest {
		d, err := sql.DigestToBytes(digest)
		if err != nil {
			sklog.Fatalf("Invalid digest %q: %s", digest, err)
		}
		m[symbol] = d
	}
	b.symbolsToDigest = m
}

func (b *SQLDataBuilder) SetGroupingKeys(fields ...string) {
	b.groupingKeys = fields
}

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

func (b *SQLDataBuilder) GenerateStructs() (schema.Tables, error) {
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
					return rv, skerr.Fmt("Duplicate trace found: %v", t.Keys)
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
	return rv, nil
}

type CommitBuilder struct {
	commits []schema.CommitRow
}

func (b *CommitBuilder) AddCommit(author, subject, commitTime string) *CommitBuilder {
	commitID := len(b.commits) + 1
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
	return b
}

type TraceBuilder struct {
	commits         []schema.CommitRow
	numTraces       int
	commonKeys      paramtools.Params
	symbolsToDigest map[rune]schema.Digest
	traceValues     []*schema.TraceValueRow
	traces          []schema.TraceRow
	groupings       []schema.GroupingRow
	options         []schema.OptionsRow
	sourceFiles     []schema.SourceFileRow
	stage           int
	groupingKeys    []string
}

func (b *TraceBuilder) History(traceHistories []string) *TraceBuilder {
	if b.stage != 0 {
		sklog.Fatalf("History must be called first and only once.")
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
	b.stage = 1
	return b
}

func (b *TraceBuilder) Keys(keys []paramtools.Params) *TraceBuilder {
	if b.stage != 1 {
		sklog.Fatalf("Keys must be called second and only once")
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

	b.stage = 2
	return b
}

func (b *TraceBuilder) OptionsAll(opts paramtools.Params) *TraceBuilder {
	xopts := make([]paramtools.Params, b.numTraces)
	for i := range xopts {
		xopts[i] = opts
	}
	return b.Options(xopts)
}

func (b *TraceBuilder) Options(xopts []paramtools.Params) *TraceBuilder {
	if b.stage == 1 {
		if b.numTraces != 1 {
			sklog.Fatalf("Must call Keys before Options")
		}
		// There is only one trace, so we can call the
		b.Keys([]paramtools.Params{{}})
	}
	if b.stage != 2 {
		sklog.Fatalf("Must call Options after Keys and not again")
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

	b.stage = 3
	return b
}

func (b *TraceBuilder) IngestedFrom(filenames, ingestedDates []string) *TraceBuilder {
	if b.stage == 0 {
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
