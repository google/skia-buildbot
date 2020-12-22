package schema

import (
	"crypto/md5"
	"time"

	"github.com/google/uuid"
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

type ExpectationLabel int

const (
	LabelUntriaged ExpectationLabel = 0
	LabelPositive  ExpectationLabel = 1
	LabelNegative  ExpectationLabel = 2
)

// Tables represents all SQL tables used by Gold. We define them as Go structs so that we can
// more easily generate test data (see sql/databuilder).
type Tables struct {
	Commits            []CommitRow
	DiffMetrics        []DiffMetricRow
	ExpectationDeltas  []ExpectationDeltaRow
	ExpectationRecords []ExpectationRecordRow
	Expectations       []ExpectationRow
	Groupings          []GroupingRow
	Options            []OptionsRow
	SourceFiles        []SourceFileRow
	TraceValues        []TraceValueRow
	Traces             []TraceRow
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

type ExpectationRecordRow struct {
	// ExpectationRecordID is a unique ID for a triage event, which could impact one or more
	// digests across one or more groupings.
	ExpectationRecordID uuid.UUID `sql:"expectation_record_id UUID PRIMARY KEY DEFAULT gen_random_uuid()"`
	// BranchName identifies to which branch the triage event happened. Can be nil for the
	// primary branch.
	BranchName *string `sql:"branch_name STRING"`
	// UserName is the email address of the logged-on user who initiated the triage event.
	UserName string `sql:"user_name STRING NOT NULL"`
	// TriageTime is the time at which this event happened.
	TriageTime time.Time `sql:"triage_time TIMESTAMP WITH TIME ZONE NOT NULL"`
	// NumChanges is how many digests were affected. It corresponds to the number of
	// ExpectationDeltaRows have this record as their parent.
	NumChanges int `sql:"num_changes INT4 NOT NULL"`
}

type ExpectationDeltaRow struct {
	// ExpectationRecordID corresponds to the parent ExpectationRecordRow.
	ExpectationRecordID uuid.UUID `sql:"expectation_record_id UUID"`
	// GroupingID identifies the grouping that was triaged by this change. This is a foreign key
	// into the Groupings table.
	GroupingID GroupingID `sql:"grouping_id BYTES"`
	// Digest is the MD5 hash of the pixel data. It identifies the image that was triaged in a
	// given grouping.
	Digest DigestBytes `sql:"digest BYTES"`
	// LabelBefore is the label that was applied to this digest in this grouping before the
	// parent expectation event happened. By storing this, we can undo that event in the future.
	LabelBefore ExpectationLabel `sql:"label_before SMALLINT NOT NULL"`
	// LabelAfter is the label that was applied as a result of the parent expectation event.
	LabelAfter ExpectationLabel `sql:"label_after SMALLINT NOT NULL"`
	// In any given expectation event, a single digest in a single grouping can only be affected
	// once, so it makes sense to use a composite primary key here. Additionally, this gives the
	// deltas good locality for a given record.
	primaryKey struct{} `sql:"PRIMARY KEY (expectation_record_id, grouping_id, digest)"`
}

// ExpectationRow contains an entry for every recent digest+grouping pair. This includes untriaged
// digests, because that allows us to have an index against label and just extract the untriaged
// ones instead of having to do a LEFT JOIN and look for nulls (which is slow at scale).
type ExpectationRow struct {
	// GroupingID identifies the grouping to which the triaged digest belongs. This is a foreign key
	// into the Groupings table.
	GroupingID GroupingID `sql:"grouping_id BYTES"`
	// Digest is the MD5 hash of the pixel data. It identifies the image that is currently triaged.
	Digest DigestBytes `sql:"digest BYTES"`
	// Label is the current label associated with the given digest in the given grouping.
	Label ExpectationLabel `sql:"label SMALLINT NOT NULL"`
	// ExpectationRecordID corresponds to most recent ExpectationRecordRow that set the given label.
	ExpectationRecordID *uuid.UUID `sql:"expectation_record_id UUID"`
	primaryKey          struct{}   `sql:"PRIMARY KEY (grouping_id, digest)"`
}

// DiffMetricRow represents the pixel-by-pixel comparison between two images (identified by their
// digests). To avoid having n^2 comparisons (where n is the number of unique digests ever seen),
// we only calculate diffs against recent images that are in the same grouping. These rows don't
// contain the grouping information for the following reasons: 1) regardless of which grouping or
// groupings an image may have been generated in, the difference between any two images is the same;
// 2) images can be produced by multiple groupings. To make certain queries easier, data for a given
// image pariing is inserted twice - once with A being Left, B being Right and once with A being
// Right and B being Left. See diff.go for more about how these fields are computed.
type DiffMetricRow struct {
	// LeftDigest represents one of the images compared.
	LeftDigest DigestBytes `sql:"left_digest BYTES"`
	// RightDigest represents the other image compared.
	RightDigest DigestBytes `sql:"left_digest BYTES"`
	// NumDiffPixels represents the number of pixels that differ between the two images.
	NumDiffPixels int `sql:"num_diff_pixels INT4 NOT NULL"`
	// PixelDiffPercent is the percentage of pixels that are different.
	PixelDiffPercent float32 `sql:"pixel_diff_percent FLOAT4 NOT NULL"`
	// MaxRGBADiffs is the maximum delta between the two images in the red, green, blue, and
	// alpha channels.
	MaxRGBADiffs [4]int `sql:"max_rgba_diffs INT2[] NOT NULL"`
	// MaxChannelDiff is max(MaxRGBADiffs). This is its own field because using max() on an array
	// field is not supported.
	MaxChannelDiff int `sql:"max_channel_diff INT2 NOT NULL"`
	// CombinedMetric is a value in [0, 10] that represents how large the diff is between two
	// images. It is based off the MaxRGBADiffs and PixelDiffPercent.
	CombinedMetric float32 `sql:"combined_metric FLOAT4 NOT NULL"`
	// DimensionsDiffer is true if the dimensions between the two images are different.
	DimensionsDiffer bool `sql:"dimensions_differ BOOL NOT NULL"`
	// Timestamp represents when this metric was computed or verified (i.e. still in use). This
	// allows for us to periodically clean up this large table.
	Timestamp  time.Time `sql:"ts TIMESTAMP WITH TIME ZONE NOT NULL"`
	primaryKey struct{}  `sql:"PRIMARY KEY (left_digest, right_digest)"`
}
