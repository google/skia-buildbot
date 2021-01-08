package schema

import (
	"crypto/md5"
	"time"

	"github.com/google/uuid"

	"go.skia.org/infra/go/paramtools"
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

type NullableBool int

func (b NullableBool) toSQL() *bool {
	var rv bool
	switch b {
	case NBNull:
		return nil
	case NBFalse:
		rv = false
	case NBTrue:
		rv = true
	default:
		return nil
	}
	return &rv
}

const (
	NBNull  NullableBool = 0
	NBFalse NullableBool = 1
	NBTrue  NullableBool = 2
)

type ExpectationLabel rune

const (
	LabelUntriaged ExpectationLabel = 'u'
	LabelPositive  ExpectationLabel = 'p'
	LabelNegative  ExpectationLabel = 'n'
)

type ChangelistStatus string

const (
	StatusOpen      ChangelistStatus = "open"
	StatusAbandoned ChangelistStatus = "abandoned"
	StatusLanded    ChangelistStatus = "landed"
)

// Tables represents all SQL tables used by Gold. We define them as Go structs so that we can
// more easily generate test data (see sql/databuilder). With the following command, the struct
// is turned into an actual SQL statement.
//go:generate go run ../exporter/tosql --output_file sql.go --logtostderr --output_pkg schema
type Tables struct {
	Changelists                 []ChangelistRow
	Commits                     []CommitRow
	DiffMetrics                 []DiffMetricRow
	ExpectationDeltas           []ExpectationDeltaRow
	ExpectationRecords          []ExpectationRecordRow
	Expectations                []ExpectationRow
	Groupings                   []GroupingRow
	IgnoreRules                 []IgnoreRuleRow
	Options                     []OptionsRow
	Patchsets                   []PatchsetRow
	PrimaryBranchParams         []PrimaryBranchParamRow
	SecondaryBranchExpectations []SecondaryBranchExpectationRow
	SecondaryBranchParams       []SecondaryBranchParamRow
	SecondaryBranchValues       []SecondaryBranchValueRow
	SourceFiles                 []SourceFileRow
	TiledTraceDigests           []TiledTraceDigestRow
	TraceValues                 []TraceValueRow
	Traces                      []TraceRow
	Tryjobs                     []TryjobRow
	ValuesAtHead                []ValueAtHeadRow

	// DeprecatedIngestedFiles allows us to keep track of files ingested with the old FS/BT ways
	// until all the SQL ingestion is ready.
	DeprecatedIngestedFiles []DeprecatedIngestedFileRow
}

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

// ToSQLRow implements the sqltest.SQLExporter interface.
func (r TraceValueRow) ToSQLRow() (colNames []string, colData []interface{}) {
	return []string{"shard", "trace_id", "commit_id", "digest", "grouping_id", "options_id", "source_file_id"},
		[]interface{}{r.Shard, r.TraceID, r.CommitID, r.Digest, r.GroupingID, r.OptionsID, r.SourceFileID}
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

// ToSQLRow implements the sqltest.SQLExporter interface.
func (r CommitRow) ToSQLRow() (colNames []string, colData []interface{}) {
	return []string{"commit_id", "git_hash", "commit_time", "author_email", "subject", "has_data"},
		[]interface{}{r.CommitID, r.GitHash, r.CommitTime, r.AuthorEmail, r.Subject, r.HasData}
}

type TraceRow struct {
	// TraceID is the MD5 hash of the keys field.
	TraceID TraceID `sql:"trace_id BYTES PRIMARY KEY"`
	// Corpus is the value associated with the "source_type" key. It is its own field for easier
	// searches and joins.
	Corpus string `sql:"corpus STRING AS (keys->>'source_type') STORED NOT NULL"`
	// GroupingID is the MD5 hash of the subset of keys that make up the grouping. It is its own
	// field for easier searches and joins. This is a foreign key into the Groupings table.
	GroupingID GroupingID `sql:"grouping_id BYTES NOT NULL"`
	// Keys is a JSON representation of a map[string]string. The keys and values of that
	// map describe how a series of data points were created. We store this as a JSON map because
	// CockroachDB supports searching for traces by key/values in this field.
	Keys paramtools.Params `sql:"keys JSONB NOT NULL"`
	// MatchesAnyIgnoreRule is true if this trace is matched by any of the ignore rules. If true,
	// this trace will be ignored from most queries by default. This is stored here because
	// recalculating it on the fly is too expensive and only needs updating if an ignore rule is
	// changed. There is a background process that computes this field for any traces with this
	// unset (i.e. NULL).
	MatchesAnyIgnoreRule NullableBool `sql:"matches_any_ignore_rule BOOL"`
}

// ToSQLRow implements the sqltest.SQLExporter interface.
func (r TraceRow) ToSQLRow() (colNames []string, colData []interface{}) {
	return []string{"trace_id", "grouping_id", "keys", "matches_any_ignore_rule"},
		[]interface{}{r.TraceID, r.GroupingID, r.Keys, r.MatchesAnyIgnoreRule.toSQL()}
}

type GroupingRow struct {
	// GroupingID is the MD5 hash of the key/values belonging to the grouping, that is, the
	// mechanism by which we partition our test data into "things that should all look the same".
	// This is commonly corpus + test name, but could include things like the color_type (e.g.
	// RGB vs greyscale).
	GroupingID GroupingID `sql:"grouping_id BYTES PRIMARY KEY"`
	// Keys is a JSON representation of a map[string]string. The keys and values of that
	// map are the grouping.
	Keys paramtools.Params `sql:"keys JSONB NOT NULL"`
}

// ToSQLRow implements the sqltest.SQLExporter interface.
func (r GroupingRow) ToSQLRow() (colNames []string, colData []interface{}) {
	return []string{"grouping_id", "keys"},
		[]interface{}{r.GroupingID, r.Keys}
}

type OptionsRow struct {
	// OptionsID is the MD5 hash of the key/values that act as metadata and do not impact the
	// uniqueness of traces.
	OptionsID OptionsID `sql:"options_id BYTES PRIMARY KEY"`
	// Keys is a JSON representation of a map[string]string. The keys and values of that
	// map are the options.
	Keys paramtools.Params `sql:"keys JSONB NOT NULL"`
}

// ToSQLRow implements the sqltest.SQLExporter interface.
func (r OptionsRow) ToSQLRow() (colNames []string, colData []interface{}) {
	return []string{"options_id", "keys"},
		[]interface{}{r.OptionsID, r.Keys}
}

type SourceFileRow struct {
	// SourceFileID is the MD5 hash of the source file that has been ingested.
	SourceFileID SourceFileID `sql:"source_file_id BYTES PRIMARY KEY"`
	// SourceFile is the fully qualified name of the source file that was ingested, e.g.
	// "gs://bucket/2020/01/02/03/15/foo.json"
	SourceFile string `sql:"source_file STRING NOT NULL"`
	// LastIngested is the time at which this file was most recently read in and successfully
	// processed.
	LastIngested time.Time `sql:"last_ingested TIMESTAMP WITH TIME ZONE NOT NULL"`
}

// ToSQLRow implements the sqltest.SQLExporter interface.
func (r SourceFileRow) ToSQLRow() (colNames []string, colData []interface{}) {
	return []string{"source_file_id", "source_file", "last_ingested"},
		[]interface{}{r.SourceFileID, r.SourceFile, r.LastIngested}
}

type DeprecatedIngestedFileRow struct {
	// SourceFileID is the MD5 hash of the source file that has been ingested.
	SourceFileID SourceFileID `sql:"source_file_id BYTES PRIMARY KEY"`
	// SourceFile is the fully qualified name of the source file that was ingested, e.g.
	// "gs://bucket/2020/01/02/03/15/foo.json"
	SourceFile string `sql:"source_file STRING NOT NULL"`
	// LastIngested is the time at which this file was most recently read in and successfully
	// processed.
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
	// ExpectationDelta rows have this record as their parent. It is a denormalized field.
	NumChanges int `sql:"num_changes INT4 NOT NULL"`
}

// ToSQLRow implements the sqltest.SQLExporter interface.
func (r ExpectationRecordRow) ToSQLRow() (colNames []string, colData []interface{}) {
	return []string{"expectation_record_id", "branch_name", "user_name", "triage_time", "num_changes"},
		[]interface{}{r.ExpectationRecordID, r.BranchName, r.UserName, r.TriageTime, r.NumChanges}
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
	LabelBefore ExpectationLabel `sql:"label_before CHAR NOT NULL"`
	// LabelAfter is the label that was applied as a result of the parent expectation event.
	LabelAfter ExpectationLabel `sql:"label_after CHAR NOT NULL"`
	// In any given expectation event, a single digest in a single grouping can only be affected
	// once, so it makes sense to use a composite primary key here. Additionally, this gives the
	// deltas good locality for a given record.
	primaryKey struct{} `sql:"PRIMARY KEY (expectation_record_id, grouping_id, digest)"`
}

// ToSQLRow implements the sqltest.SQLExporter interface.
func (r ExpectationDeltaRow) ToSQLRow() (colNames []string, colData []interface{}) {
	return []string{"expectation_record_id", "grouping_id", "digest", "label_before", "label_after"},
		[]interface{}{r.ExpectationRecordID, r.GroupingID, r.Digest, string(r.LabelBefore), string(r.LabelAfter)}
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
	Label ExpectationLabel `sql:"label CHAR NOT NULL"`
	// ExpectationRecordID corresponds to most recent ExpectationRecordRow that set the given label.
	ExpectationRecordID *uuid.UUID `sql:"expectation_record_id UUID"`
	primaryKey          struct{}   `sql:"PRIMARY KEY (grouping_id, digest)"`
}

// ToSQLRow implements the sqltest.SQLExporter interface.
func (r ExpectationRow) ToSQLRow() (colNames []string, colData []interface{}) {
	return []string{"grouping_id", "digest", "label", "expectation_record_id"},
		[]interface{}{r.GroupingID, r.Digest, string(r.Label), r.ExpectationRecordID}
}

// DiffMetricRow represents the pixel-by-pixel comparison between two images (identified by their
// digests). To avoid having n^2 comparisons (where n is the number of unique digests ever seen),
// we only calculate diffs against recent images that are in the same grouping. These rows don't
// contain the grouping information for the following reasons: 1) regardless of which grouping or
// groupings an image may have been generated in, the difference between any two images is the same;
// 2) images can be produced by multiple groupings. To make certain queries easier, data for a given
// image pairing is inserted twice - once with A being Left, B being Right and once with A being
// Right and B being Left. See diff.go for more about how these fields are computed.
type DiffMetricRow struct {
	// LeftDigest represents one of the images compared.
	LeftDigest DigestBytes `sql:"left_digest BYTES"`
	// RightDigest represents the other image compared.
	RightDigest DigestBytes `sql:"right_digest BYTES"`
	// NumPixelsDiff represents the number of pixels that differ between the two images.
	NumPixelsDiff int `sql:"num_pixels_diff INT4 NOT NULL"`
	// PercentPixelsDiff is the percentage of pixels that are different.
	PercentPixelsDiff float32 `sql:"percent_pixels_diff FLOAT4 NOT NULL"`
	// MaxRGBADiffs is the maximum delta between the two images in the red, green, blue, and
	// alpha channels.
	MaxRGBADiffs [4]int `sql:"max_rgba_diffs INT2[] NOT NULL"`
	// MaxChannelDiff is max(MaxRGBADiffs). This is its own field because using max() on an array
	// field is not supported. TODO(kjlubick) could this be computed with array_upper()?
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

// ToSQLRow implements the sqltest.SQLExporter interface.
func (r DiffMetricRow) ToSQLRow() (colNames []string, colData []interface{}) {
	return []string{"left_digest", "right_digest", "num_pixels_diff", "percent_pixels_diff", "max_rgba_diffs",
			"max_channel_diff", "combined_metric", "dimensions_differ", "ts"},
		[]interface{}{r.LeftDigest, r.RightDigest, r.NumPixelsDiff, r.PercentPixelsDiff, r.MaxRGBADiffs,
			r.MaxChannelDiff, r.CombinedMetric, r.DimensionsDiffer, r.Timestamp}
}

// ValueAtHeadRow represents the most recent data point for a each trace. It contains some
// denormalized data to reduce the number of joins needed to do some frequent queries.
type ValueAtHeadRow struct {
	// TraceID is the MD5 hash of the keys and values describing how this data point (i.e. the
	// Digest) was drawn. This is a foreign key into the Traces table.
	TraceID TraceID `sql:"trace_id BYTES PRIMARY KEY"`
	// MostRecentCommitID represents when in time this data was drawn. This is a foreign key into
	// the CommitIDs table.
	MostRecentCommitID CommitID `sql:"most_recent_commit_id INT4 NOT NULL"`
	// Digest is the MD5 hash of the pixel data; this is "what was drawn" at this point in time
	// (specified by MostRecentCommitID) and by the machine (specified by TraceID).
	Digest DigestBytes `sql:"digest BYTES NOT NULL"`
	// OptionsID is the MD5 hash of the key/values that belong to the options. Options do not impact
	// the TraceID and thus act as metadata. This is a foreign key into the Options table.
	OptionsID OptionsID `sql:"options_id BYTES NOT NULL"`
	// GroupingID is the MD5 hash of the key/values belonging to the grouping (e.g. corpus +
	// test name).

	GroupingID GroupingID `sql:"grouping_id BYTES NOT NULL"`
	// Corpus is the value associated with the "source_type" key. It is its own field for easier
	// searches and joins.
	Corpus string `sql:"corpus STRING AS (keys->>'source_type') STORED NOT NULL"`
	// Keys is a JSON representation of a map[string]string that are the trace keys.
	Keys paramtools.Params `sql:"keys JSONB NOT NULL"`

	// Label represents the current triage status of the given digest for its grouping.
	Label ExpectationLabel `sql:"expectation_label CHAR NOT NULL"`
	// ExpectationRecordID (if set) is the record ID of the triage record. This allows fast lookup
	// of who triaged this when.
	ExpectationRecordID *uuid.UUID `sql:"expectation_record_id UUID"`
	// MatchesAnyIgnoreRule is true if this trace is matched by any of the ignore rules.
	MatchesAnyIgnoreRule NullableBool `sql:"matches_any_ignore_rule BOOL"`
}

// ToSQLRow implements the sqltest.SQLExporter interface.
func (r ValueAtHeadRow) ToSQLRow() (colNames []string, colData []interface{}) {
	return []string{"trace_id", "most_recent_commit_id", "digest", "options_id", "grouping_id",
			"keys", "expectation_label", "expectation_record_id", "matches_any_ignore_rule"},
		[]interface{}{r.TraceID, r.MostRecentCommitID, r.Digest, r.OptionsID, r.GroupingID,
			r.Keys, string(r.Label), r.ExpectationRecordID, r.MatchesAnyIgnoreRule.toSQL()}
}

// PrimaryBranchParamRow corresponds to a given key/value pair that was seen within a range (tile)
// of commits. Originally, we had done a join between Traces and TraceValues to find the params
// where commit_id was in a given range. However, this took several minutes when there were 1M+
// traces per commit. This table is effectively an index for getting just that data. Note, this
// contains Keys *and* Options because users can search/filter/query by both of those and this table
// is used to fill in the UI widgets with the available search options.
type PrimaryBranchParamRow struct {
	// StartCommitID is the commit id that is the beginning of the tile for which this row
	// corresponds. For example, with a tile width of 100, data from commit 73 would correspond to
	// StartCommitID == 0; data from commit 1234 would correspond with StartCommitID == 1200 and
	// so on. This is a foreign key into the CommitIDs table.
	StartCommitID CommitID `sql:"start_commit_id INT4"`
	// Key is the key of a trace key or option.
	Key string `sql:"key STRING"`
	// Value is the value associated with the key.
	Value string `sql:"value STRING"`
	// We generally want locality by tile, so that goes first in the primary key.
	primaryKey struct{} `sql:"PRIMARY KEY (start_commit_id, key, value)"`
}

// ToSQLRow implements the sqltest.SQLExporter interface.
func (r PrimaryBranchParamRow) ToSQLRow() (colNames []string, colData []interface{}) {
	return []string{"start_commit_id", "key", "value"},
		[]interface{}{r.StartCommitID, r.Key, r.Value}
}

// TiledTraceDigestRow corresponds to a given trace producing a given digest within a range (tile)
// of commits. Originally, we did a SELECT DISTINCT over TraceValues, but that was too slow for
// many queries when the number of TraceValues was high.
type TiledTraceDigestRow struct {
	// TraceID is the MD5 hash of the keys and values describing how this data point (i.e. the
	// Digest) was drawn. This is a foreign key into the Traces table.
	TraceID TraceID `sql:"trace_id BYTES"`
	// StartCommitID is the commit id that is the beginning of the tile for which this row
	// corresponds.
	StartCommitID CommitID `sql:"start_commit_id INT4"`
	// Digest is the MD5 hash of the pixel data; this is "what was drawn" at least once in the tile
	// specified by StartCommitID and by the machine (specified by TraceID).
	Digest DigestBytes `sql:"digest BYTES NOT NULL"`
	// We generally want locality by TraceID, so that goes first in the primary key.
	primaryKey struct{} `sql:"PRIMARY KEY (trace_id, start_commit_id, digest)"`
}

// ToSQLRow implements the sqltest.SQLExporter interface.
func (r TiledTraceDigestRow) ToSQLRow() (colNames []string, colData []interface{}) {
	return []string{"trace_id", "start_commit_id", "digest"},
		[]interface{}{r.TraceID, r.StartCommitID, r.Digest}
}

type IgnoreRuleRow struct {
	// IgnoreRuleID is the id for this rule.
	IgnoreRuleID uuid.UUID `sql:"ignore_rule_id UUID PRIMARY KEY DEFAULT gen_random_uuid()"`
	// CreatorEmail is the email address of the user who originally created this rule.
	CreatorEmail string `sql:"creator_email STRING NOT NULL"`
	// UpdatedEmail is the email address of the user who most recently updated this rule.
	UpdatedEmail string `sql:"updated_email STRING NOT NULL"`
	// Expires represents when this rule should be re-evaluated for validity.
	Expires time.Time `sql:"expires TIMESTAMP WITH TIME ZONE"`
	// Note is a comment explaining this rule. It typically links to a bug.
	Note string `sql:"note STRING"`
	// Query is a map[string][]string that describe which traces should be ignored.
	// Note that this can only apply to trace keys, not options.
	Query paramtools.ReadOnlyParamSet `sql:"query JSONB"`
}

// ToSQLRow implements the sqltest.SQLExporter interface.
func (r IgnoreRuleRow) ToSQLRow() (colNames []string, colData []interface{}) {
	return []string{"ignore_rule_id", "creator_email", "updated_email", "expires", "note", "query"},
		[]interface{}{r.IgnoreRuleID, r.CreatorEmail, r.UpdatedEmail, r.Expires, r.Note, r.Query}
}

type ChangelistRow struct {
	// ChangelistID is the fully qualified id of this changelist. "Fully qualified" means it has
	// the system as a prefix (e.g "gerrit_1234") which simplifies joining logic and ensures
	// uniqueness
	ChangelistID string `sql:"changelist_id STRING PRIMARY KEY"`
	// System is the Code Review System to which this changelist belongs.
	System string `sql:"system STRING NOT NULL"`
	// Status indicates if this CL is open or not.
	Status ChangelistStatus `sql:"status STRING NOT NULL"`
	// OwnerEmail is the email address of the CL's owner.
	OwnerEmail string `sql:"owner_email STRING NOT NULL"`
	// Subject is the first line of the CL's commit message (usually).
	Subject string `sql:"subject STRING NOT NULL"`
	// LastIngestedData indicates when Gold last saw data for this CL.
	LastIngestedData time.Time `sql:"last_ingested_data TIMESTAMP WITH TIME ZONE NOT NULL"`
}

// ToSQLRow implements the sqltest.SQLExporter interface.
func (r ChangelistRow) ToSQLRow() (colNames []string, colData []interface{}) {
	return []string{"changelist_id", "system", "status", "owner_email", "subject", "last_ingested_data"},
		[]interface{}{r.ChangelistID, r.System, r.Status, r.OwnerEmail, r.Subject, r.LastIngestedData}
}

type PatchsetRow struct {
	// PatchsetID is the fully qualified id of this patchset. "Fully qualified" means it has
	// the system as a prefix (e.g "gerrit_abcde") which simplifies joining logic and ensures
	// uniqueness.
	PatchsetID string `sql:"patchset_id STRING PRIMARY KEY"`
	// System is the Code Review System to which this patchset belongs.
	System string `sql:"system STRING NOT NULL"`
	// ChangelistID refers to the parent CL.
	ChangelistID string `sql:"changelist_id STRING NOT NULL REFERENCES Changelists (changelist_id)"`
	// Order is a 1 indexed number telling us where this PS fits in time.
	Order int `sql:"ps_order INT2 NOT NULL"`
	// GitHash is the hash associated with the patchset. For many CRS, it is the same as the
	// unqualified PatchsetID.
	GitHash string `sql:"git_hash STRING NOT NULL"`
}

// ToSQLRow implements the sqltest.SQLExporter interface.
func (r PatchsetRow) ToSQLRow() (colNames []string, colData []interface{}) {
	return []string{"patchset_id", "system", "changelist_id", "ps_order", "git_hash"},
		[]interface{}{r.PatchsetID, r.System, r.ChangelistID, r.Order, r.GitHash}
}

type TryjobRow struct {
	// PatchsetID is the fully qualified id of this patchset. "Fully qualified" means it has
	// the system as a prefix (e.g "buildbucket_1234") which simplifies joining logic and ensures
	// uniqueness.
	TryjobID string `sql:"tryjob_id STRING PRIMARY KEY"`
	// System is the Continuous Integration System to which this tryjob belongs.
	System string `sql:"system STRING NOT NULL"`
	// ChangelistID refers to the CL for which this Tryjob produced data.
	ChangelistID string `sql:"changelist_id STRING NOT NULL REFERENCES Changelists (changelist_id)"`
	// PatchsetID refers to the PS for which this Tryjob produced data.
	PatchsetID string `sql:"patchset_id STRING NOT NULL REFERENCES Patchsets (patchset_id)"`
	// DisplayName is a human readable name for this Tryjob.
	DisplayName string `sql:"display_name STRING NOT NULL"`
	// LastIngestedData indicates when Gold last saw data from this Tryjob.
	LastIngestedData time.Time `sql:"last_ingested_data TIMESTAMP WITH TIME ZONE NOT NULL"`
}

// ToSQLRow implements the sqltest.SQLExporter interface.
func (r TryjobRow) ToSQLRow() (colNames []string, colData []interface{}) {
	return []string{"tryjob_id", "system", "changelist_id", "patchset_id", "display_name", "last_ingested_data"},
		[]interface{}{r.TryjobID, r.System, r.ChangelistID, r.PatchsetID, r.DisplayName, r.LastIngestedData}
}

// SecondaryBranchValueRow corresponds to a data point produced by a changelist or on a branch.
type SecondaryBranchValueRow struct {
	// BranchName is a something like "gerrit_12345" or "chrome_m86" to identify the branch.
	BranchName string `sql:"branch_name STRING"`
	// VersionName is something like the patchset id or a branch commit hash to identify when
	// along the branch the data happened.
	VersionName string `sql:"version_name STRING"`
	// TraceID is the MD5 hash of the keys and values describing how this data point (i.e. the
	// Digest) was drawn. This is a foreign key into the Traces table.
	TraceID TraceID `sql:"secondary_branch_trace_id BYTES"`
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
	// TryjobID corresponds to the tryjob (if any) that produced this data.
	TryjobID string `sql:"tryjob_id string"`
	// By creating the primary key using the shard and the commit_id, we give some data locality
	// to data from the same trace, but in different commits w/o overloading a single range (if
	// commit_id were first) and w/o spreading our data too thin (if trace_id were first).
	primaryKey struct{} `sql:"PRIMARY KEY (branch_name, version_name, secondary_branch_trace_id)"`
}

// ToSQLRow implements the sqltest.SQLExporter interface.
func (r SecondaryBranchValueRow) ToSQLRow() (colNames []string, colData []interface{}) {
	return []string{"branch_name", "version_name", "secondary_branch_trace_id", "digest", "grouping_id",
			"options_id", "source_file_id", "tryjob_id"},
		[]interface{}{r.BranchName, r.VersionName, r.TraceID, r.Digest, r.GroupingID,
			r.OptionsID, r.SourceFileID, r.TryjobID}
}

// SecondaryBranchParamRow corresponds to a given key/value pair that was seen in data from a
// specific patchset or commit on a branch.
type SecondaryBranchParamRow struct {
	// BranchName is a something like "gerrit_12345" or "chrome_m86" to identify the branch.
	BranchName string `sql:"branch_name STRING"`
	// VersionName is something like the patchset id or a branch commit hash to identify when
	// along the branch the data happened.
	VersionName string `sql:"version_name STRING"`
	// Key is the key of a trace key or option.
	Key string `sql:"key STRING"`
	// Value is the value associated with the key.
	Value string `sql:"value STRING"`
	// We generally want locality by branch_name, so that goes first in the primary key.
	primaryKey struct{} `sql:"PRIMARY KEY (branch_name, version_name, key, value)"`
}

// ToSQLRow implements the sqltest.SQLExporter interface.
func (r SecondaryBranchParamRow) ToSQLRow() (colNames []string, colData []interface{}) {
	return []string{"branch_name", "version_name", "key", "value"},
		[]interface{}{r.BranchName, r.VersionName, r.Key, r.Value}
}

// SecondaryBranchExpectationRow responds to a new expectation rule applying to a single Changelist.
// We save expectations per Changelist to avoid the extra effort having having to re-triage
// everything if a new Patchset was updated that fixes something slightly.
type SecondaryBranchExpectationRow struct {
	// BranchName is a something like "gerrit_12345" or "chrome_m86" to identify the branch.
	BranchName string `sql:"branch_name STRING"`
	// GroupingID identifies the grouping to which the triaged digest belongs. This is a foreign key
	// into the Groupings table.
	GroupingID GroupingID `sql:"grouping_id BYTES"`
	// Digest is the MD5 hash of the pixel data. It identifies the image that is currently triaged.
	Digest DigestBytes `sql:"digest BYTES"`
	// Label is the current label associated with the given digest in the given grouping.
	Label ExpectationLabel `sql:"label CHAR NOT NULL"`
	// ExpectationRecordID corresponds to most recent ExpectationRecordRow that set the given label.
	// Unlike the primary branch, this can never be nil/null because we only keep track of
	// secondary branch expectations for triaged events.
	ExpectationRecordID uuid.UUID `sql:"expectation_record_id UUID NOT NULL"`
	primaryKey          struct{}  `sql:"PRIMARY KEY (branch_name, grouping_id, digest)"`
}

// ToSQLRow implements the sqltest.SQLExporter interface.
func (r SecondaryBranchExpectationRow) ToSQLRow() (colNames []string, colData []interface{}) {
	return []string{"branch_name", "grouping_id", "digest", "label", "expectation_record_id"},
		[]interface{}{r.BranchName, r.GroupingID, r.Digest, string(r.Label), r.ExpectationRecordID}
}
