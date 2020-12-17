package testutils

import (
	"fmt"
	"strings"
	"time"

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
			panic(err)
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
		panic("Must add commits before traces")
	}
	if len(b.groupingKeys) == 0 {
		panic("Must add grouping keys before traces")
	}
	if len(b.symbolsToDigest) == 0 {
		panic("Must add digests before traces")
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

type CommitBuilder struct {
	commits []schema.CommitRow
}

func (b *CommitBuilder) AddCommit(author, subject string, commitTime time.Time) *CommitBuilder {
	commitID := len(b.commits) + 1
	gitHash := fmt.Sprintf("%04d%", commitID)
	// A true githash is 40 hex characters, so we repeat the 4 digits of the commitID 10 times.
	gitHash = strings.Repeat(gitHash, 10)
	b.commits = append(b.commits, schema.CommitRow{
		CommitID:   schema.CommitID(commitID),
		GitHash:    gitHash,
		CommitTime: commitTime,
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
		panic("History must be called first and only once.")
	}
	b.numTraces = len(traceHistories)
	// traceValues will have length len(commits) * numTraces after this is complete. Some entries
	// may be nil to represent "no data" and will be stripped out later.
	for _, th := range traceHistories {
		if len(th) != len(b.commits) {
			panic(fmt.Sprintf("history %q is of invalid length: expected %d", th, len(b.commits)))
		}
		for i, symbol := range th {
			if symbol == '-' {
				b.traceValues = append(b.traceValues, nil)
				continue
			}
			digest, ok := b.symbolsToDigest[symbol]
			if !ok {
				panic("Unknown symbol in trace history " + string(symbol))
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
		panic("Keys must be called second and only once")
	}
	if len(keys) != b.numTraces {
		panic("Expected one set of keys for each trace")
	}
	// We now have enough data to make all the traces.
	groupings := map[schema.SerializedJSON]schema.GroupingID{}
	seenTraces := map[schema.SerializedJSON]bool{}
	for i, traceParams := range keys {
		traceParams.Add(b.commonKeys)
		grouping := make(map[string]string, len(keys))
		for _, gk := range b.groupingKeys {
			val, ok := traceParams[gk]
			if !ok {
				panic(fmt.Sprintf("Missing grouping key %q from %v", gk, traceParams))
			}
			grouping[gk] = val
		}
		groupingJSON, groupingID := sql.SerializeMap(grouping)
		groupings[groupingJSON] = groupingID

		traceJSON, traceID := sql.SerializeMap(traceParams)
		if seenTraces[traceJSON] {
			panic("Found identical trace" + traceJSON)
		}
		numCommits := len(b.commits)
		for _, row := range b.traceValues[i*numCommits : (i+1)*numCommits] {
			if row != nil {
				row.GroupingID = groupingID
				row.TraceID = traceID
			}
		}
	}
nextNewGrouping:
	for groupingJSON, groupingID := range groupings {
		// Check to make sure we don't have duplicates
		for _, g := range b.groupings {
			if g.Keys == groupingJSON {
				continue nextNewGrouping
			}
		}
		b.groupings = append(b.groupings, schema.GroupingRow{
			GroupingID: groupingID,
			Keys:       groupingJSON,
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
			panic("Must call Keys before Options")
		}
		// There is only one trace, so we can call the
		b.Keys([]paramtools.Params{{}})
	}
	if b.stage != 2 {
		panic("Must call Options after Keys and not again")
	}
	options := map[schema.SerializedJSON]schema.OptionsID{}
	for i, tv := range b.traceValues {
		optionsJSON, optionsID := sql.SerializeMap(xopts[i])
		tv.OptionsID = optionsID
		options[optionsJSON] = optionsID
	}

nextNewOption:
	for optJSON, optID := range options {
		// Check to make sure we don't have duplicates
		for _, g := range b.options {
			if g.Keys == optJSON {
				continue nextNewOption
			}
		}
		b.options = append(b.options, schema.OptionsRow{
			OptionsID: optID,
			Keys:      optJSON,
		})
	}
	b.stage = 3
	return b
}

func (b *TraceBuilder) IngestedFrom(filenames, ingestedDates []string) {
	for i, tv := range b.traceValues {

	}
}
