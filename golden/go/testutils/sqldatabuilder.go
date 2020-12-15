package testutils

import (
	"fmt"
	"strings"
	"time"

	"go.skia.org/infra/golden/go/sql/schema"
)

type SQLDataBuilder struct {
	commitBuilder *CommitBuilder
}

func (b *SQLDataBuilder) DenseHistory() *CommitBuilder {
	b.commitBuilder = &CommitBuilder{}
	return b.commitBuilder
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
