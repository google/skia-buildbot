package schema

import (
	"crypto/md5"
	"time"
)

type MD5Hash [md5.Size]byte
type CommitID int32
type TraceID []byte
type GroupingID []byte
type OptionsID []byte
type Digest []byte
type SourceFileID []byte

type Tables struct {
	TraceValues []TraceValueRow
	Commits     []CommitRow
}

type TraceValueRow struct {
	// Shard is a small piece of the trace id to slightly break up trace data.
	Shard byte `sql:"shard INT2"`
	// TraceID is the MD5 hash of the keys and values describing how this data point (i.e. the
	// Digest) was drawn. This is a foreign key into the Traces table.
	TraceID TraceID `sql:"trace_id BYTES"`
	// CommitID represents when in time this data was drawn. This is a foreign key into the
	// CommitIDs table.
	CommitID CommitID `sql:"commit_id INT4"`
	// Digest is the MD5 hash of the pixel data; this is "what was drawn" at this point in time
	// (specified by CommitID) and by the machine (specified by TraceID).
	Digest Digest `sql:"digest BYTES NOT NULL"`
	// GroupingID is the MD5 hash of the key/values belonging to the grouping (e.g. corpus +
	// test name). If the grouping changes, this would require altering the table. In theory,
	// changing the grouping should be done very rarely. This is a foreign key into the Groupings
	// table.
	GroupingID GroupingID `sql:"grouping_id BYTES NOT NULL"`
	// OptionsID is the MD5 hash of the key/values that belong to the options. Options do not impact
	// the TraceID and thus act as metadata. This is a foreign key into the Options table.
	OptionsID OptionsID `sql:"options_id BYTES NOT NULL"`
	// SourceFileID is the MD5 hash of the source file that produced this data point. This is a
	// foreign key into the SourceFiles table.
	SourceFileID SourceFileID `sql:"source_file_id BYTES NOT NULL"`
	// By creating the primary key using the shard and the commit_id, we give some data locality
	// to data from the same trace, but in different commits w/o overloading a single range (if
	// commit_id were first) and w/o spreading our data too thin (if trace_id were first).
	primaryKey struct{} `sql:"PRIMARY KEY (shard, commit_id, trace_id)"`
}

type CommitRow struct {
	// CommitID is a monotonically increasing number as we follow the primary repo through time.
	CommitID CommitID `sql:"commit_id INT4 PRIMARY KEY"`
	// GitHash is the git hash of the commit.
	GitHash string `sql:"git_hash STRING NOT NULL"`
	// CommitTime is the timestamp associated with the commit.
	CommitTime time.Time `sql:"commit_time TIMESTAMP WITH TIME ZONE NOT NULL"`
	// Author is the email address associated with the author.
	Author string `sql:"author STRING NOT NULL"`
	// Subject is the subject line of the commit.
	Subject string `sql:"subject STRING NOT NULL"`
	// HasData is set the first time data lands on the primary branch for this commit number. We
	// use this to determine the dense tile of data. Previously, we had tried to determine this
	// with a DISTINCT search over TraceValues, but that takes several minutes when there are
	// 1M+ traces per commit.
	HasData bool `sql:"has_data BOOL"`
}
