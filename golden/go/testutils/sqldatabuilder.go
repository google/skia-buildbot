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

func (b *SQLDataBuilder) TracesWithCommonKeys(params paramtools.Params) *TraceBuilder {
	if len(b.commitBuilder.commits) == 0 {
		panic("Must add commits before traces")
	}
	tb := &TraceBuilder{
		commits:         b.commitBuilder.commits,
		commonKeys:      params,
		symbolsToDigest: b.symbolsToDigest,
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

func (b *TraceBuilder) Keys(keys []paramtools.Params) {
	if b.stage != 1 {
		panic("Keys must be called second and only once")
	}
	// We now have enough data to make all the traces.
}
