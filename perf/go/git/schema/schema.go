package schema

import "go.skia.org/infra/perf/go/types"

// Commit represents a single commit stored in the database.
//
// JSON annotations make it serialize like the legacy cid.CommitDetail.
type Commit struct {
	CommitNumber types.CommitNumber `sql:"commit_number INT PRIMARY KEY"`
	GitHash      string             `sql:"git_hash TEXT UNIQUE NOT NULL"`
	Timestamp    int64              `sql:"commit_time INT"` // Unix timestamp, seconds from the epoch.
	Author       string             `sql:"author TEXT"`
	Subject      string             `sql:"subject TEXT"`
}
