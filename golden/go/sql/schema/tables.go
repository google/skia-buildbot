package schema

import (
	"crypto/md5"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgtype"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/golden/go/expectations"
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

// CommitID is responsible for indicating when a commit happens in time. CommitIDs should be
// treated as ordered lexicographically, but need not be densely populated.
type CommitID string

// TileID is a mechanism for batching together data to speed up queries. A tile will consist of a
// configurable number of commits with data.
type TileID int

type NullableBool int

// ToSQL returns the nullable value as a type compatible with SQL backends.
func (b NullableBool) ToSQL() *bool {
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

func toNullableBool(matches pgtype.Bool) NullableBool {
	if matches.Status == pgtype.Present {
		if matches.Bool {
			return NBTrue
		} else {
			return NBFalse
		}
	}
	return NBNull
}

const (
	NBNull  NullableBool = 0
	NBFalse NullableBool = 1
	NBTrue  NullableBool = 2
)

type ExpectationLabel string

const (
	LabelUntriaged ExpectationLabel = "u"
	LabelPositive  ExpectationLabel = "p"
	LabelNegative  ExpectationLabel = "n"
)

func (e ExpectationLabel) ToExpectation() expectations.Label {
	switch e {
	case LabelPositive:
		return expectations.Positive
	case LabelNegative:
		return expectations.Negative
	case LabelUntriaged:
		return expectations.Untriaged
	}
	return expectations.Untriaged
}

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
	Changelists                 []ChangelistRow                 `sql_backup:"weekly"`
	CommitsWithData             []CommitWithDataRow             `sql_backup:"daily"`
	DiffMetrics                 []DiffMetricRow                 `sql_backup:"monthly"`
	ExpectationDeltas           []ExpectationDeltaRow           `sql_backup:"daily"`
	ExpectationRecords          []ExpectationRecordRow          `sql_backup:"daily"`
	Expectations                []ExpectationRow                `sql_backup:"daily"`
	GitCommits                  []GitCommitRow                  `sql_backup:"daily"`
	Groupings                   []GroupingRow                   `sql_backup:"monthly"`
	IgnoreRules                 []IgnoreRuleRow                 `sql_backup:"daily"`
	MetadataCommits             []MetadataCommitRow             `sql_backup:"daily"`
	Options                     []OptionsRow                    `sql_backup:"monthly"`
	Patchsets                   []PatchsetRow                   `sql_backup:"weekly"`
	PrimaryBranchParams         []PrimaryBranchParamRow         `sql_backup:"monthly"`
	ProblemImages               []ProblemImageRow               `sql_backup:"none"`
	SecondaryBranchExpectations []SecondaryBranchExpectationRow `sql_backup:"daily"`
	SecondaryBranchParams       []SecondaryBranchParamRow       `sql_backup:"monthly"`
	SecondaryBranchValues       []SecondaryBranchValueRow       `sql_backup:"monthly"`
	SourceFiles                 []SourceFileRow                 `sql_backup:"monthly"`
	TiledTraceDigests           []TiledTraceDigestRow           `sql_backup:"monthly"`
	TrackingCommits             []TrackingCommitRow             `sql_backup:"daily"`
	TraceValues                 []TraceValueRow                 `sql_backup:"monthly"`
	Traces                      []TraceRow                      `sql_backup:"monthly"`
	Tryjobs                     []TryjobRow                     `sql_backup:"weekly"`
	ValuesAtHead                []ValueAtHeadRow                `sql_backup:"monthly"`

	// DeprecatedIngestedFiles allows us to keep track of files ingested with the old FS/BT ways
	// until all the SQL ingestion is ready.
	DeprecatedIngestedFiles    []DeprecatedIngestedFileRow    `sql_backup:"daily"`
	DeprecatedExpectationUndos []DeprecatedExpectationUndoRow `sql_backup:"daily"`
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
	CommitID CommitID `sql:"commit_id STRING"`
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

	traceCommitIndex struct{} `sql:"INDEX trace_commit_idx (trace_id, commit_id) STORING (digest, options_id, grouping_id)"`
}

// ToSQLRow implements the sqltest.SQLExporter interface.
func (r TraceValueRow) ToSQLRow() (colNames []string, colData []interface{}) {
	return []string{"shard", "trace_id", "commit_id", "digest", "grouping_id", "options_id", "source_file_id"},
		[]interface{}{r.Shard, r.TraceID, r.CommitID, r.Digest, r.GroupingID, r.OptionsID, r.SourceFileID}
}

// ScanFrom implements the sqltest.SQLScanner interface.
func (r *TraceValueRow) ScanFrom(scan func(...interface{}) error) error {
	return scan(&r.Shard, &r.TraceID, &r.CommitID, &r.Digest, &r.GroupingID,
		&r.OptionsID, &r.SourceFileID)
}

// CommitWithDataRow represents a commit that has produced some data on the primary branch.
// It is expected to be created during ingestion.
type CommitWithDataRow struct {
	// CommitID is a potentially arbitrary string. commit_ids will be treated as occurring in
	// lexicographical order.
	CommitID CommitID `sql:"commit_id STRING PRIMARY KEY"`
	// TileID is an integer that corresponds to the tile for which this commit belongs. Tiles are
	// intended to be about 100 commits wide, but this could vary slightly if commits are ingested
	// in not quite sequential order. Additionally, tile widths could change over time if necessary.
	// It is expected that tile_id be set the first time we see data from a given commit on the
	// primary branch and not changed after, even if the tile size used for an instance changes.
	TileID TileID `sql:"tile_id INT4 NOT NULL"`
}

// ToSQLRow implements the sqltest.SQLExporter interface.
func (r CommitWithDataRow) ToSQLRow() (colNames []string, colData []interface{}) {
	return []string{"commit_id", "tile_id"},
		[]interface{}{r.CommitID, r.TileID}
}

// ScanFrom implements the sqltest.SQLScanner interface.
func (r *CommitWithDataRow) ScanFrom(scan func(...interface{}) error) error {
	return scan(&r.CommitID, &r.TileID)
}

// RowsOrderBy implements the sqltest.RowsOrder interface to sort commits by CommitID.
func (r CommitWithDataRow) RowsOrderBy() string {
	return `ORDER BY commit_id ASC`
}

// GitCommitRow represents a git commit that we may or may not have seen data for.
type GitCommitRow struct {
	// GitHash is the git hash of the commit.
	GitHash string `sql:"git_hash STRING PRIMARY KEY"`
	// CommitID is a potentially arbitrary string. It is a foreign key in the CommitsWithData table.
	CommitID CommitID `sql:"commit_id STRING NOT NULL"`
	// CommitTime is the timestamp associated with the commit.
	CommitTime time.Time `sql:"commit_time TIMESTAMP WITH TIME ZONE NOT NULL"`
	// AuthorEmail is the email address associated with the author.
	AuthorEmail string `sql:"author_email STRING NOT NULL"`
	// Subject is the subject line of the commit.
	Subject string `sql:"subject STRING NOT NULL"`

	commitIDIndex struct{} `sql:"INDEX commit_idx (commit_id)"`
}

// ToSQLRow implements the sqltest.SQLExporter interface.
func (r GitCommitRow) ToSQLRow() (colNames []string, colData []interface{}) {
	return []string{"git_hash", "commit_id", "commit_time", "author_email", "subject"},
		[]interface{}{r.GitHash, r.CommitID, r.CommitTime, r.AuthorEmail, r.Subject}
}

// ScanFrom implements the sqltest.SQLScanner interface.
func (r *GitCommitRow) ScanFrom(scan func(...interface{}) error) error {
	if err := scan(&r.GitHash, &r.CommitID, &r.CommitTime, &r.AuthorEmail, &r.Subject); err != nil {
		return skerr.Wrap(err)
	}
	r.CommitTime = r.CommitTime.UTC()
	return nil
}

// RowsOrderBy implements the sqltest.RowsOrder interface to sort commits by CommitID.
func (r GitCommitRow) RowsOrderBy() string {
	return `ORDER BY commit_id DESC`
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

	// This index speeds up fetching traces by grouping, e.g. when enumerating the work needed for
	// creating diffs.
	groupingIgnoredIndex struct{} `sql:"INDEX grouping_ignored_idx (grouping_id, matches_any_ignore_rule)"`
	// This index makes application of all ignore rules easier.
	ignoredGroupingIndex struct{} `sql:"INDEX ignored_grouping_idx (matches_any_ignore_rule, grouping_id)"`
	// This index makes querying by keys faster
	keysIndex struct{} `sql:"INVERTED INDEX keys_idx (keys)"`
}

// ToSQLRow implements the sqltest.SQLExporter interface.
func (r TraceRow) ToSQLRow() (colNames []string, colData []interface{}) {
	return []string{"trace_id", "grouping_id", "keys", "matches_any_ignore_rule"},
		[]interface{}{r.TraceID, r.GroupingID, r.Keys, r.MatchesAnyIgnoreRule.ToSQL()}
}

// ScanFrom implements the sqltest.SQLScanner interface.
func (r *TraceRow) ScanFrom(scan func(...interface{}) error) error {
	var matches pgtype.Bool
	if err := scan(&r.TraceID, &r.Corpus, &r.GroupingID, &r.Keys, &matches); err != nil {
		return skerr.Wrap(err)
	}
	r.MatchesAnyIgnoreRule = toNullableBool(matches)
	return nil
}

// RowsOrderBy implements the sqltest.RowsOrder interface to sort traces by test name.
func (r TraceRow) RowsOrderBy() string {
	return `ORDER BY keys->>'name'`
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

// ScanFrom implements the sqltest.SQLScanner interface.
func (r *GroupingRow) ScanFrom(scan func(...interface{}) error) error {
	return scan(&r.GroupingID, &r.Keys)
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

// ScanFrom implements the sqltest.SQLScanner interface.
func (r *OptionsRow) ScanFrom(scan func(...interface{}) error) error {
	return scan(&r.OptionsID, &r.Keys)
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

// ScanFrom implements the sqltest.SQLScanner interface.
func (r *SourceFileRow) ScanFrom(scan func(...interface{}) error) error {
	if err := scan(&r.SourceFileID, &r.SourceFile, &r.LastIngested); err != nil {
		return skerr.Wrap(err)
	}
	r.LastIngested = r.LastIngested.UTC()
	return nil
}

// RowsOrderBy implements the sqltest.RowsOrder interface.
func (r SourceFileRow) RowsOrderBy() string {
	return "ORDER BY last_ingested ASC"
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

// ScanFrom implements the sqltest.SQLScanner interface.
func (r *DeprecatedIngestedFileRow) ScanFrom(scan func(...interface{}) error) error {
	if err := scan(&r.SourceFileID, &r.SourceFile, &r.LastIngested); err != nil {
		return skerr.Wrap(err)
	}
	r.LastIngested = r.LastIngested.UTC()
	return nil
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
	NumChanges         int      `sql:"num_changes INT4 NOT NULL"`
	branchTriagedIndex struct{} `sql:"INDEX branch_ts_idx (branch_name, triage_time)"`
}

// ToSQLRow implements the sqltest.SQLExporter interface.
func (r ExpectationRecordRow) ToSQLRow() (colNames []string, colData []interface{}) {
	return []string{"expectation_record_id", "branch_name", "user_name", "triage_time", "num_changes"},
		[]interface{}{r.ExpectationRecordID, r.BranchName, r.UserName, r.TriageTime, r.NumChanges}
}

// ScanFrom implements the sqltest.SQLScanner interface.
func (r *ExpectationRecordRow) ScanFrom(scan func(...interface{}) error) error {
	err := scan(&r.ExpectationRecordID, &r.BranchName, &r.UserName, &r.TriageTime, &r.NumChanges)
	if err != nil {
		return skerr.Wrap(err)
	}
	r.TriageTime = r.TriageTime.UTC()
	return nil
}

// RowsOrderBy implements the sqltest.RowsOrder interface, sorting rows to have the most recent
// records first, with ties broken by num_changes and user_name
func (r ExpectationRecordRow) RowsOrderBy() string {
	return "ORDER BY triage_time DESC, num_changes DESC, user_name ASC"
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
		[]interface{}{r.ExpectationRecordID, r.GroupingID, r.Digest, r.LabelBefore, r.LabelAfter}
}

// ScanFrom implements the sqltest.SQLScanner interface.
func (r *ExpectationDeltaRow) ScanFrom(scan func(...interface{}) error) error {
	return scan(&r.ExpectationRecordID, &r.GroupingID, &r.Digest, &r.LabelBefore, &r.LabelAfter)
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
	labelIndex          struct{}   `sql:"INDEX label_idx (label)"`
}

// ToSQLRow implements the sqltest.SQLExporter interface.
func (r ExpectationRow) ToSQLRow() (colNames []string, colData []interface{}) {
	return []string{"grouping_id", "digest", "label", "expectation_record_id"},
		[]interface{}{r.GroupingID, r.Digest, r.Label, r.ExpectationRecordID}
}

// ScanFrom implements the sqltest.SQLScanner interface.
func (r *ExpectationRow) ScanFrom(scan func(...interface{}) error) error {
	return scan(&r.GroupingID, &r.Digest, &r.Label, &r.ExpectationRecordID)
}

// RowsOrderBy implements the sqltest.RowsOrder interface, sorting the rows first by digest, then
// by grouping id (which is a hash).
func (r ExpectationRow) RowsOrderBy() string {
	return "ORDER BY digest, grouping_id ASC"
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

// ScanFrom implements the sqltest.SQLScanner interface.
func (r *DiffMetricRow) ScanFrom(scan func(...interface{}) error) error {
	err := scan(&r.LeftDigest, &r.RightDigest, &r.NumPixelsDiff, &r.PercentPixelsDiff,
		&r.MaxRGBADiffs, &r.MaxChannelDiff, &r.CombinedMetric, &r.DimensionsDiffer, &r.Timestamp)
	if err != nil {
		return skerr.Wrap(err)
	}
	r.Timestamp = r.Timestamp.UTC()
	return nil
}

// ValueAtHeadRow represents the most recent data point for a each trace. It contains some
// denormalized data to reduce the number of joins needed to do some frequent queries.
type ValueAtHeadRow struct {
	// TraceID is the MD5 hash of the keys and values describing how this data point (i.e. the
	// Digest) was drawn. This is a foreign key into the Traces table.
	TraceID TraceID `sql:"trace_id BYTES PRIMARY KEY"`
	// MostRecentCommitID represents when in time this data was drawn. This is a foreign key into
	// the CommitIDs table.
	MostRecentCommitID CommitID `sql:"most_recent_commit_id STRING NOT NULL"`
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

	// MatchesAnyIgnoreRule is true if this trace is matched by any of the ignore rules.
	MatchesAnyIgnoreRule NullableBool `sql:"matches_any_ignore_rule BOOL"`

	// This index makes application of all ignore rules easier.
	ignoredGroupingIndex struct{} `sql:"INDEX ignored_grouping_idx (matches_any_ignore_rule, grouping_id)"`
	// This index makes searching for recent untriaged digests faster. The STORING clause is
	// important to not have to do a lookup after finding the item in the index.
	corpusCommitIgnoreIndex struct{} `sql:"INDEX corpus_commit_ignore_idx (corpus, most_recent_commit_id, matches_any_ignore_rule) STORING (grouping_id, digest)"`
	// This index makes querying by keys faster
	keysIndex struct{} `sql:"INVERTED INDEX keys_idx (keys)"`
}

// ToSQLRow implements the sqltest.SQLExporter interface.
func (r ValueAtHeadRow) ToSQLRow() (colNames []string, colData []interface{}) {
	return []string{"trace_id", "most_recent_commit_id", "digest", "options_id", "grouping_id",
			"keys", "matches_any_ignore_rule"},
		[]interface{}{r.TraceID, r.MostRecentCommitID, r.Digest, r.OptionsID, r.GroupingID,
			r.Keys, r.MatchesAnyIgnoreRule.ToSQL()}
}

// ScanFrom implements the sqltest.SQLScanner interface.
func (r *ValueAtHeadRow) ScanFrom(scan func(...interface{}) error) error {
	var matches pgtype.Bool
	err := scan(&r.TraceID, &r.MostRecentCommitID, &r.Digest, &r.OptionsID,
		&r.GroupingID, &r.Corpus, &r.Keys, &matches)
	if err != nil {
		return skerr.Wrap(err)
	}
	r.MatchesAnyIgnoreRule = toNullableBool(matches)
	return nil
}

// PrimaryBranchParamRow corresponds to a given key/value pair that was seen within a range (tile)
// of commits. Originally, we had done a join between Traces and TraceValues to find the params
// where commit_id was in a given range. However, this took several minutes when there were 1M+
// traces per commit. This table is effectively an index for getting just that data. Note, this
// contains Keys *and* Options because users can search/filter/query by both of those and this table
// is used to fill in the UI widgets with the available search options.
type PrimaryBranchParamRow struct {
	// TileID indicates which tile the given Key and Value were seen on in the primary branch.
	// This is a foreign key into the Commits table.
	TileID TileID `sql:"tile_id INT4"`
	// Key is the key of a trace key or option.
	Key string `sql:"key STRING"`
	// Value is the value associated with the key.
	Value string `sql:"value STRING"`
	// We generally want locality by tile, so that goes first in the primary key.
	primaryKey struct{} `sql:"PRIMARY KEY (tile_id, key, value)"`
}

// ToSQLRow implements the sqltest.SQLExporter interface.
func (r PrimaryBranchParamRow) ToSQLRow() (colNames []string, colData []interface{}) {
	return []string{"tile_id", "key", "value"},
		[]interface{}{r.TileID, r.Key, r.Value}
}

// ScanFrom implements the sqltest.SQLScanner interface.
func (r *PrimaryBranchParamRow) ScanFrom(scan func(...interface{}) error) error {
	return scan(&r.TileID, &r.Key, &r.Value)
}

// RowsOrderBy implements the sqltest.RowsOrder interface.
func (r PrimaryBranchParamRow) RowsOrderBy() string {
	return `ORDER BY tile_id, key ASC`
}

// TiledTraceDigestRow corresponds to a given trace producing a given digest within a range (tile)
// of commits. Originally, we did a SELECT DISTINCT over TraceValues, but that was too slow for
// many queries when the number of TraceValues was high.
type TiledTraceDigestRow struct {
	// TraceID is the MD5 hash of the keys and values describing how this data point (i.e. the
	// Digest) was drawn. This is a foreign key into the Traces table.
	TraceID TraceID `sql:"trace_id BYTES"`
	// TileID represents the tile for this row.
	TileID TileID `sql:"tile_id INT4"`
	// Digest is the MD5 hash of the pixel data; this is "what was drawn" at least once in the tile
	// specified by StartCommitID and by the machine (specified by TraceID).
	Digest DigestBytes `sql:"digest BYTES NOT NULL"`
	// GroupingID is the grouping of the trace.
	GroupingID GroupingID `sql:"grouping_id BYTES NOT NULL"`
	// We generally want locality by TraceID, so that goes first in the primary key.
	primaryKey struct{} `sql:"PRIMARY KEY (trace_id, tile_id, digest)"`

	// This index makes it easier to answer the question "What digests are being produced by
	// a given grouping on the primary branch).
	groupingDigestIndex struct{} `sql:"INDEX grouping_digest_idx (grouping_id, digest)"`
	tileTraceIndex      struct{} `sql:"INDEX tile_trace_idx (tile_id, trace_id)"`
}

// ToSQLRow implements the sqltest.SQLExporter interface.
func (r TiledTraceDigestRow) ToSQLRow() (colNames []string, colData []interface{}) {
	return []string{"trace_id", "tile_id", "digest", "grouping_id"},
		[]interface{}{r.TraceID, r.TileID, r.Digest, r.GroupingID}
}

// ScanFrom implements the sqltest.SQLScanner interface.
func (r *TiledTraceDigestRow) ScanFrom(scan func(...interface{}) error) error {
	return scan(&r.TraceID, &r.TileID, &r.Digest, &r.GroupingID)
}

// RowsOrderBy implements the sqltest.RowsOrder interface.
func (r TiledTraceDigestRow) RowsOrderBy() string {
	return `ORDER BY tile_id, digest ASC`
}

type IgnoreRuleRow struct {
	// IgnoreRuleID is the id for this rule.
	IgnoreRuleID uuid.UUID `sql:"ignore_rule_id UUID PRIMARY KEY DEFAULT gen_random_uuid()"`
	// CreatorEmail is the email address of the user who originally created this rule.
	CreatorEmail string `sql:"creator_email STRING NOT NULL"`
	// UpdatedEmail is the email address of the user who most recently updated this rule.
	UpdatedEmail string `sql:"updated_email STRING NOT NULL"`
	// Expires represents when this rule should be re-evaluated for validity.
	Expires time.Time `sql:"expires TIMESTAMP WITH TIME ZONE NOT NULL"`
	// Note is a comment explaining this rule. It typically links to a bug.
	Note string `sql:"note STRING"`
	// Query is a map[string][]string that describe which traces should be ignored.
	// Note that this can only apply to trace keys, not options.
	Query paramtools.ReadOnlyParamSet `sql:"query JSONB NOT NULL"`
}

// ToSQLRow implements the sqltest.SQLExporter interface.
func (r IgnoreRuleRow) ToSQLRow() (colNames []string, colData []interface{}) {
	return []string{"ignore_rule_id", "creator_email", "updated_email", "expires", "note", "query"},
		[]interface{}{r.IgnoreRuleID, r.CreatorEmail, r.UpdatedEmail, r.Expires, r.Note, r.Query}
}

// ScanFrom implements the sqltest.SQLScanner interface.
func (r *IgnoreRuleRow) ScanFrom(scan func(...interface{}) error) error {
	if err := scan(&r.IgnoreRuleID, &r.CreatorEmail, &r.UpdatedEmail, &r.Expires, &r.Note, &r.Query); err != nil {
		return skerr.Wrap(err)
	}
	r.Expires = r.Expires.UTC()
	paramtools.ParamSet(r.Query).Normalize()
	return nil
}

// RowsOrderBy implements the sqltest.RowsOrder interface.
func (r IgnoreRuleRow) RowsOrderBy() string {
	return `ORDER BY expires ASC`
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

	// This index helps query for recently updated, open CLs. Keep an eye on this index, as it could
	// lead to hotspotting: https://www.cockroachlabs.com/docs/v20.2/indexes.html#indexing-columns
	systemStatusIngestedIndex struct{} `sql:"INDEX system_status_ingested_idx (system, status, last_ingested_data)"`
}

// ToSQLRow implements the sqltest.SQLExporter interface.
func (r ChangelistRow) ToSQLRow() (colNames []string, colData []interface{}) {
	return []string{"changelist_id", "system", "status", "owner_email", "subject", "last_ingested_data"},
		[]interface{}{r.ChangelistID, r.System, r.Status, r.OwnerEmail, r.Subject, r.LastIngestedData}
}

// ScanFrom implements the sqltest.SQLScanner interface.
func (r *ChangelistRow) ScanFrom(scan func(...interface{}) error) error {
	if err := scan(&r.ChangelistID, &r.System, &r.Status, &r.OwnerEmail, &r.Subject, &r.LastIngestedData); err != nil {
		return skerr.Wrap(err)
	}
	r.LastIngestedData = r.LastIngestedData.UTC()
	return nil
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
	// CommentedOnCL keeps track of if Gold has commented on the CL indicating there are digests
	// that need human attention (e.g. there are non-flaky, untriaged, and unignored digests).
	// We should comment on a CL at most once per Patchset.
	CommentedOnCL bool `sql:"commented_on_cl BOOL NOT NULL"`
	// LastCheckedIfCommentNecessary remembers when we last queried the data for this PS to see
	// if it needed a comment. It is used to avoid searching the database if there have been
	// no updates to the CL since the last time we looked.
	LastCheckedIfCommentNecessary time.Time `sql:"last_checked_if_comment_necessary TIMESTAMP WITH TIME ZONE NOT NULL"`

	clOrderIndex struct{} `sql:"INDEX cl_order_idx (changelist_id, ps_order)"`
}

// ToSQLRow implements the sqltest.SQLExporter interface.
func (r PatchsetRow) ToSQLRow() (colNames []string, colData []interface{}) {
	return []string{"patchset_id", "system", "changelist_id", "ps_order", "git_hash",
			"commented_on_cl", "last_checked_if_comment_necessary"},
		[]interface{}{r.PatchsetID, r.System, r.ChangelistID, r.Order, r.GitHash,
			r.CommentedOnCL, r.LastCheckedIfCommentNecessary}
}

// ScanFrom implements the sqltest.SQLScanner interface.
func (r *PatchsetRow) ScanFrom(scan func(...interface{}) error) error {
	err := scan(&r.PatchsetID, &r.System, &r.ChangelistID, &r.Order, &r.GitHash,
		&r.CommentedOnCL, &r.LastCheckedIfCommentNecessary)
	if err != nil {
		return skerr.Wrap(err)
	}
	r.LastCheckedIfCommentNecessary = r.LastCheckedIfCommentNecessary.UTC()
	return nil
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

// ScanFrom implements the sqltest.SQLScanner interface.
func (r *TryjobRow) ScanFrom(scan func(...interface{}) error) error {
	err := scan(&r.TryjobID, &r.System, &r.ChangelistID, &r.PatchsetID, &r.DisplayName, &r.LastIngestedData)
	if err != nil {
		return skerr.Wrap(err)
	}
	r.LastIngestedData = r.LastIngestedData.UTC()
	return nil
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
	// TryjobID corresponds to the latest tryjob (if any) that produced this data. When/if we
	// support branches, this may be null, e.g. data coming from chrome_m86.
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

// ScanFrom implements the sqltest.SQLScanner interface.
func (r *SecondaryBranchValueRow) ScanFrom(scan func(...interface{}) error) error {
	return scan(&r.BranchName, &r.VersionName, &r.TraceID, &r.Digest, &r.GroupingID,
		&r.OptionsID, &r.SourceFileID, &r.TryjobID)
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

// ScanFrom implements the sqltest.SQLScanner interface.
func (r *SecondaryBranchParamRow) ScanFrom(scan func(...interface{}) error) error {
	return scan(&r.BranchName, &r.VersionName, &r.Key, &r.Value)
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

// ScanFrom implements the sqltest.SQLScanner interface.
func (r *SecondaryBranchExpectationRow) ScanFrom(scan func(...interface{}) error) error {
	return scan(&r.BranchName, &r.GroupingID, &r.Digest, &r.Label, &r.ExpectationRecordID)
}

type ProblemImageRow struct {
	// Digest is the identifier of an image we had a hard time downloading or decoding while
	// computing the diffs. This is a string because it may be a malformed digest.
	Digest string `sql:"digest STRING PRIMARY KEY"`
	// NumErrors counts the number of times this digest has been the cause of an error.
	NumErrors int `sql:"num_errors INT2 NOT NULL"`
	// LatestError is the string content of the last error associated with this digest.
	LatestError string `sql:"latest_error STRING NOT NULL"`
	// ErrorTS is the last time we had an error on this digest.
	ErrorTS time.Time `sql:"error_ts TIMESTAMP WITH TIME ZONE NOT NULL"`
}

// ToSQLRow implements the sqltest.SQLExporter interface.
func (r ProblemImageRow) ToSQLRow() (colNames []string, colData []interface{}) {
	return []string{"digest", "num_errors", "latest_error", "error_ts"},
		[]interface{}{r.Digest, r.NumErrors, r.Digest, r.ErrorTS}
}

// ScanFrom implements the sqltest.SQLScanner interface.
func (r *ProblemImageRow) ScanFrom(scan func(...interface{}) error) error {
	if err := scan(&r.Digest, &r.NumErrors, &r.LatestError, &r.ErrorTS); err != nil {
		return skerr.Wrap(err)
	}
	r.ErrorTS = r.ErrorTS.UTC()
	return nil
}

// RowsOrderBy implements the sqltest.RowsOrder interface.
func (r ProblemImageRow) RowsOrderBy() string {
	return `ORDER BY digest ASC`
}

// DeprecatedExpectationUndoRow represents an undo operation that we could not automatically
// apply during the transitional period of expectations. A human will manually apply these when
// removing the firestore implementation from the loop.
type DeprecatedExpectationUndoRow struct {
	ID            int       `sql:"id SERIAL PRIMARY KEY"`
	ExpectationID string    `sql:"expectation_id STRING NOT NULL"`
	UserID        string    `sql:"user_id STRING NOT NULL"`
	TS            time.Time `sql:"ts TIMESTAMP WITH TIME ZONE NOT NULL"`
}

// ScanFrom implements the sqltest.SQLScanner interface.
func (r *DeprecatedExpectationUndoRow) ScanFrom(scan func(...interface{}) error) error {
	if err := scan(&r.ID, &r.ExpectationID, &r.UserID, &r.TS); err != nil {
		return skerr.Wrap(err)
	}
	r.TS = r.TS.UTC()
	return nil
}

// TrackingCommitRow represents a repo for which we have checked to see if the commits landed
// correspond to any Changelists.
type TrackingCommitRow struct {
	// Repo is the url of the repo we are tracking
	Repo string `sql:"repo STRING PRIMARY KEY"`
	// LastGitHash is the git hash of the commit that we know landed most recently.
	LastGitHash string `sql:"last_git_hash STRING NOT NULL"`
}

// ToSQLRow implements the sqltest.SQLExporter interface.
func (r TrackingCommitRow) ToSQLRow() (colNames []string, colData []interface{}) {
	return []string{"repo", "last_git_hash"},
		[]interface{}{r.Repo, r.LastGitHash}
}

// ScanFrom implements the sqltest.SQLScanner interface.
func (r *TrackingCommitRow) ScanFrom(scan func(...interface{}) error) error {
	return scan(&r.Repo, &r.LastGitHash)
}

type MetadataCommitRow struct {
	// CommitID is a potentially arbitrary string. It is a foreign key in the CommitsWithData table.
	CommitID CommitID `sql:"commit_id STRING PRIMARY KEY"`
	// CommitMetadata is an arbitrary string; For current implementations, it is a link to a GCS
	// file that has more information about the state of the repo when the data was generated.
	CommitMetadata string `sql:"commit_metadata STRING NOT NULL"`
}

// ToSQLRow implements the sqltest.SQLExporter interface.
func (r MetadataCommitRow) ToSQLRow() (colNames []string, colData []interface{}) {
	return []string{"commit_id", "commit_metadata"},
		[]interface{}{r.CommitID, r.CommitMetadata}
}

// ScanFrom implements the sqltest.SQLScanner interface.
func (r *MetadataCommitRow) ScanFrom(scan func(...interface{}) error) error {
	return scan(&r.CommitID, &r.CommitMetadata)
}
