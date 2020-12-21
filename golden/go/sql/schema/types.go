package schema

import (
	"crypto/md5"
	"time"
)

// MD5Hash is a specialized type for an array of bytes representing an MD5Hash. We use MD5 hashes
// a lot because they are a deterministic way to generate primary keys, which allows us to more
// easily cache and deduplicate data when ingesting. Additionally, these hashes are somewhat
// space-efficient (compared to UUIDs) and allow us to not have too big of data when
// cross-referencing other tables.
type MD5Hash [md5.Size]byte

// TraceID and the other related types are declared as byte slices instead of MD5Hash because
// 1) we want to avoid copying around the the array data every time (reminder: in Golang, arrays
//   are passed by value); and
// 2) passing arrays into the pgx driver is a little awkward (it only accepts slices so we would
//   have to turn the arrays into slices before passing in and if we forget, it's a runtime error).
type TraceID []byte
type GroupingID []byte
type OptionsID []byte
type DigestBytes []byte
type SourceFileID []byte

type CommitID int32

// SerializedJSON is the string form of a JSON-encoded map[string]string. Following the convention
// of the golang json encoder, keys must be in alphabetical order (for determinism).
type SerializedJSON string

type NullableBool int

const (
	NBNull  NullableBool = 0
	NBFalse NullableBool = 1
	NBTrue  NullableBool = 2
)

// Tables represents all SQL tables used by Gold. We define them as Go structs so that we can
// more easily generate test data (see sqldatabuilder).
type Tables struct {
	TraceValues []TraceValueRow
	Commits     []CommitRow
	Traces      []TraceRow
	Groupings   []GroupingRow
	Options     []OptionsRow
	SourceFiles []SourceFileRow
}

// TODO(kjlubick) add code to generate SQL statements from these struct tags
type TraceValueRow struct {
	// Shard is a small piece of the trace id to slightly break up trace data. TODO(kjlubick) could
	//   this be a computed column?
	Shard byte `sql:"shard INT2"`
	// TraceID is the MD5 hash of the keys and values describing how this data point (i.e. the
	// Digest) was drawn. This is a foreign key into the Traces table.
	TraceID TraceID `sql:"trace_id BYTES"`
	// CommitID represents when in time this data was drawn. This is a foreign key into the
	// CommitIDs table.
	CommitID CommitID `sql:"commit_id INT4"`
	// Digest is the MD5 hash of the pixel data; this is "what was drawn" at this point in time
	// (specified by CommitID) and by the machine (specified by TraceID).
	Digest DigestBytes `sql:"digest BYTES NOT NULL"`
	// GroupingID is the MD5 hash of the key/values belonging to the grouping (e.g. corpus +
	// test name). If the grouping changes, this would require changing the entire column here and
	// in several other tables. In theory, changing the grouping should be done very rarely. This
	// is a foreign key into the Groupings table.
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
	// AuthorEmail is the email address associated with the author.
	AuthorEmail string `sql:"author_email STRING NOT NULL"`
	// Subject is the subject line of the commit.
	Subject string `sql:"subject STRING NOT NULL"`
	// HasData is set the first time data lands on the primary branch for this commit number. We
	// use this to determine the dense tile of data. Previously, we had tried to determine this
	// with a DISTINCT search over TraceValues, but that takes several minutes when there are
	// 1M+ traces per commit.
	HasData bool `sql:"has_data BOOL NOT NULL"`
}

type TraceRow struct {
	// TraceID is the MD5 hash of the keys field.
	TraceID TraceID `sql:"trace_id BYTES PRIMARY KEY"`
	// Corpus is the value associated with the "source_type" key. It is its own field for easier
	// searches and joins.
	Corpus string `sql:"corpus STRING AS (keys->>'id') STORED NOT NULL"`
	// GroupingID is the MD5 hash of the subset of keys that make up the grouping. It is its own
	// field for easier searches and joins. This is a foreign key into the Groupings table.
	GroupingID GroupingID `sql:"grouping_id BYTES NOT NULL"`
	// Keys is a serialized JSON representation of a map[string]string. The keys and values of that
	// map describe how a series of data points were created. We store this as a JSON map because
	// CockroachDB supports searching for traces by key/values in this field.
	Keys SerializedJSON `sql:"keys JSONB NOT NULL"`
	// MatchesAnyIgnoreRule is true if this trace is matched by any of the ignore rules. If true,
	// this trace will be ignored from most queries by default. This is stored here because
	// recalculating it on the fly is too expensive and only needs updating if an ignore rule is
	// changed. There is a background process that computes this field for any traces with this
	// unset (i.e. NULL).
	MatchesAnyIgnoreRule NullableBool `sql:"matches_any_ignore_rule BOOL"`
}

type GroupingRow struct {
	// GroupingID is the MD5 hash of the key/values belonging to the grouping, that is, the
	// mechanism by which we partition our test data into "things that should all look the same".
	// This is commonly corpus + test name, but could include things like the color_type (e.g.
	// RGB vs greyscale).
	GroupingID GroupingID `sql:"grouping_id BYTES PRIMARY KEY"`
	// Keys is a serialized JSON representation of a map[string]string. The keys and values of that
	// map are the grouping.
	Keys SerializedJSON `sql:"keys JSONB NOT NULL"`
}

type OptionsRow struct {
	// OptionsID is the MD5 hash of the key/values that act as metadata and do not impact the
	// uniqueness of traces.
	OptionsID OptionsID `sql:"options_id BYTES PRIMARY KEY"`
	// Keys is a serialized JSON representation of a map[string]string. The keys and values of that
	// map are the options.
	Keys SerializedJSON `sql:"keys JSONB NOT NULL"`
}

type SourceFileRow struct {
	// SourceFileID is the MD5 hash of the source file that has been ingested.
	SourceFileID SourceFileID `sql:"source_file_id BYTES PRIMARY KEY"`
	// SourceFile is the fully qualified name of the source file that was ingested, e.g.
	// "gs://bucket/2020/01/02/03/15/foo.json"
	SourceFile string `sql:"source_file STRING NOT NULL"`
	// LastIngested is the time at which this file was most recently read in.
	LastIngested time.Time `sql:"last_ingested TIMESTAMP WITH TIME ZONE NOT NULL"`
}
