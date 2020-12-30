// Package databuilder provides a tool for generating test data in a way that is easy for
// a human to update and understand.
package databuilder

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"image"
	"image/png"
	"io"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/types"
)

// TablesBuilder has methods on it for generating trace data and other related data in a way
// that can be easily turned into SQL table rows.
type TablesBuilder struct {
	changelistBuilders  []*ChangelistBuilder
	commitBuilder       *CommitBuilder
	diffMetrics         []schema.DiffMetricRow
	expectationBuilders []*ExpectationsBuilder
	groupingKeys        []string
	ignoreRules         []schema.IgnoreRuleRow
	runeToDigest        map[rune]schema.DigestBytes
	tileWidth           int
	traceBuilders       []*TraceBuilder
}

// Commits returns a new CommitBuilder linked to this builder which will have a set. It panics if
// called more than once.
func (b *TablesBuilder) Commits() *CommitBuilder {
	if b.commitBuilder != nil {
		logAndPanic("Cannot call Commits() more than once.")
	}
	b.commitBuilder = &CommitBuilder{}
	return b.commitBuilder
}

// SetDigests loads a mapping of runes to the digest that they represent. This allows
// specifying the trace history be done with a string of characters. If a rune is invalid or
// the digests are invalid, this will panic. It panics if called more than once.
func (b *TablesBuilder) SetDigests(runeToDigest map[rune]types.Digest) {
	if b.runeToDigest != nil {
		logAndPanic("Cannot call SetDigests() more than once.")
	}
	m := make(map[rune]schema.DigestBytes, len(runeToDigest))
	for symbol, digest := range runeToDigest {
		if symbol == '-' {
			logAndPanic("Cannot map something to -")
		}
		d, err := sql.DigestToBytes(digest)
		if err != nil {
			logAndPanic("Invalid digest %q: %s", digest, err)
		}
		m[symbol] = d
	}
	b.runeToDigest = m
}

// SetGroupingKeys specifies which keys from a Trace's params will be used to define the grouping.
// It panics if called more than once.
func (b *TablesBuilder) SetGroupingKeys(fields ...string) {
	if b.groupingKeys != nil {
		logAndPanic("Cannot call SetGroupingKeys() more than once.")
	}
	b.groupingKeys = fields
}

// AddTracesWithCommonKeys returns a new TraceBuilder for building a set of related traces. This can
// be called more than once - all data will be combined at the end. It panics if any of its
// prerequisites have not been called.
func (b *TablesBuilder) AddTracesWithCommonKeys(params paramtools.Params) *TraceBuilder {
	if b.commitBuilder == nil {
		logAndPanic("Must add commits before traces")
	}
	if len(b.commitBuilder.commits) == 0 {
		logAndPanic("Must specify at least one commit")
	}
	if len(b.groupingKeys) == 0 {
		logAndPanic("Must add grouping keys before traces")
	}
	if len(b.runeToDigest) == 0 {
		logAndPanic("Must add digests before traces")
	}
	tb := &TraceBuilder{
		commits:         b.commitBuilder.commits,
		commonKeys:      params,
		symbolsToDigest: b.runeToDigest,
		groupingKeys:    b.groupingKeys,
	}
	b.traceBuilders = append(b.traceBuilders, tb)
	return tb
}

// AddTriageEvent returns a builder for a series of triage events on the primary branch. These
// events will be attributed to the given user and timestamp.
func (b *TablesBuilder) AddTriageEvent(user, triageTime string) *ExpectationsBuilder {
	if len(b.groupingKeys) == 0 {
		logAndPanic("Must add grouping keys before expectations")
	}
	ts, err := time.Parse(time.RFC3339, triageTime)
	if err != nil {
		logAndPanic("Invalid triage time %q: %s", triageTime, err)
	}
	eb := &ExpectationsBuilder{
		groupingKeys: b.groupingKeys,
		record: &schema.ExpectationRecordRow{
			ExpectationRecordID: uuid.New(),
			BranchName:          nil,
			UserName:            user,
			TriageTime:          ts,
		},
	}
	b.expectationBuilders = append(b.expectationBuilders, eb)
	return eb
}

// finalizeExpectations finds the final state of the expectations for each digest+grouping pair.
// This includes identifying untriaged digests. It panics if any of the ExpectationDeltas have
// preconditions.
func (b *TablesBuilder) finalizeExpectations() []*schema.ExpectationRow {
	var expectations []*schema.ExpectationRow
	find := func(g schema.GroupingID, d schema.DigestBytes) *schema.ExpectationRow {
		for _, e := range expectations {
			if bytes.Equal(e.GroupingID, g) && bytes.Equal(e.Digest, d) {
				return e
			}
		}
		return nil
	}
	for _, eb := range b.expectationBuilders {
		eb.record.NumChanges = len(eb.deltas)
		for _, ed := range eb.deltas {
			existingRecord := find(ed.GroupingID, ed.Digest)
			if existingRecord == nil {
				expectations = append(expectations, &schema.ExpectationRow{
					GroupingID:          ed.GroupingID,
					Digest:              ed.Digest,
					Label:               ed.LabelAfter,
					ExpectationRecordID: &ed.ExpectationRecordID,
				})
				continue
			}
			if existingRecord.Label != ed.LabelBefore {
				logAndPanic("Expectation Delta precondition is incorrect. Label before was %d: %#v", existingRecord.Label, ed)
			}
		}
	}
	// Fill in untriaged values.
	for _, tb := range b.traceBuilders {
		for _, trace := range tb.traceValues {
			for _, tv := range trace {
				if tv == nil {
					continue
				}
				existingRecord := find(tv.GroupingID, tv.Digest)
				if existingRecord == nil {
					expectations = append(expectations, &schema.ExpectationRow{
						GroupingID: tv.GroupingID,
						Digest:     tv.Digest,
						Label:      schema.LabelUntriaged,
					})
				}
			}
		}
	}
	return expectations
}

// ComputeDiffMetricsFromImages generates all the diff metrics from the observed images/digests
// and from the trace history. It reads the images from disk to use in order to compute the diffs.
// It is expected that the images are in the provided directory named [digest].png.
func (b *TablesBuilder) ComputeDiffMetricsFromImages(imgDir string, nowStr string) {
	if len(b.diffMetrics) != 0 {
		logAndPanic("Must call ComputeDiffMetricsFromImages only once")
	}
	if len(b.runeToDigest) == 0 {
		logAndPanic("Must call ComputeDiffMetricsFromImages after SetDigests")
	}
	if len(b.traceBuilders) == 0 {
		logAndPanic("Must call ComputeDiffMetricsFromImages after inserting trace data")
	}
	now, err := time.Parse(time.RFC3339, nowStr)
	if err != nil {
		logAndPanic("Invalid time for now: %s", err)
	}
	_, err = ioutil.ReadDir(imgDir)
	if err != nil {
		logAndPanic("Error reading directory %q: %s", imgDir, err)
	}
	images := make(map[types.Digest]*image.NRGBA, len(b.runeToDigest))
	for _, db := range b.runeToDigest {
		d := types.Digest(hex.EncodeToString(db))
		images[d] = openNRGBAFromDisk(imgDir, d)
	}

	// Maps groupingID to digests that appear in that grouping across all the trace data.
	toCompute := map[schema.MD5Hash]types.DigestSet{}
	for _, tb := range b.traceBuilders {
		for _, trace := range tb.traceValues {
			for _, tv := range trace {
				if tv != nil {
					groupingID := sql.AsMD5Hash(tv.GroupingID)
					if _, ok := toCompute[groupingID]; !ok {
						toCompute[groupingID] = types.DigestSet{}
					}
					d := types.Digest(hex.EncodeToString(tv.Digest))
					toCompute[groupingID][d] = true
				}
			}
		}
	}
	// Add all the data from the CLs to the respective groupings.
	for _, clb := range b.changelistBuilders {
		for _, psb := range clb.patchsets {
			for _, ps := range psb.dataPoints {
				for _, dp := range ps {
					if dp != nil {
						groupingID := sql.AsMD5Hash(dp.GroupingID)
						if _, ok := toCompute[groupingID]; !ok {
							toCompute[groupingID] = types.DigestSet{}
						}
						d := types.Digest(hex.EncodeToString(dp.Digest))
						toCompute[groupingID][d] = true
					}
				}
			}
		}
	}
	// For each grouping, compare each digest to every other digest and create the metric rows
	// for that.
	for _, xd := range toCompute {
		digests := xd.Keys()
		sort.Sort(digests)
		for leftIdx, leftDigest := range digests {
			leftImg := images[leftDigest]
			for rightIdx := leftIdx + 1; rightIdx < len(digests); rightIdx++ {
				rightDigest := digests[rightIdx]
				rightImg := images[rightDigest]
				dm := diff.ComputeDiffMetrics(leftImg, rightImg)
				if dm.NumDiffPixels == 0 {
					logAndPanic("%s and %s aren't different", leftDigest, rightDigest)
				}
				ld, err := sql.DigestToBytes(leftDigest)
				if err != nil {
					logAndPanic(err.Error())
				}
				rd, err := sql.DigestToBytes(rightDigest)
				if err != nil {
					logAndPanic(err.Error())
				}
				b.diffMetrics = append(b.diffMetrics, schema.DiffMetricRow{
					LeftDigest:        ld,
					RightDigest:       rd,
					NumPixelsDiff:     dm.NumDiffPixels,
					PercentPixelsDiff: dm.PixelDiffPercent,
					MaxRGBADiffs:      dm.MaxRGBADiffs,
					MaxChannelDiff:    max(dm.MaxRGBADiffs),
					CombinedMetric:    dm.CombinedMetric,
					DimensionsDiffer:  dm.DimDiffer,
					Timestamp:         now,
				})
				// And in the other order of left-right
				b.diffMetrics = append(b.diffMetrics, schema.DiffMetricRow{
					LeftDigest:        rd,
					RightDigest:       ld,
					NumPixelsDiff:     dm.NumDiffPixels,
					PercentPixelsDiff: dm.PixelDiffPercent,
					MaxRGBADiffs:      dm.MaxRGBADiffs,
					MaxChannelDiff:    max(dm.MaxRGBADiffs),
					CombinedMetric:    dm.CombinedMetric,
					DimensionsDiffer:  dm.DimDiffer,
					Timestamp:         now,
				})
			}
		}
	}
}

func max(d [4]int) int {
	m := -1
	for _, k := range d {
		if k > m {
			m = k
		}
	}
	return m
}

func openNRGBAFromDisk(basePath string, digest types.Digest) *image.NRGBA {
	var img *image.NRGBA
	path := filepath.Join(basePath, string(digest)+".png")
	err := util.WithReadFile(path, func(r io.Reader) error {
		im, err := png.Decode(r)
		if err != nil {
			return skerr.Wrapf(err, "decoding %s", path)
		}
		img = diff.GetNRGBA(im)
		return nil
	})
	if err != nil {
		logAndPanic(err.Error())
	}
	return img
}

// AddIgnoreRule adds an ignore rule with the given information. It will be applied to traces during
// the generation of structs.
func (b *TablesBuilder) AddIgnoreRule(created, updated, updateTS, note string, query paramtools.ParamSet) uuid.UUID {
	ts, err := time.Parse(time.RFC3339, updateTS)
	if err != nil {
		logAndPanic("invalid time %q: %s", updateTS, err)
	}
	if len(query) == 0 {
		logAndPanic("Cannot use empty rule")
	}

	id := uuid.New()
	b.ignoreRules = append(b.ignoreRules, schema.IgnoreRuleRow{
		IgnoreRuleID: id,
		CreatorEmail: created,
		UpdatedEmail: updated,
		Expires:      ts,
		Note:         note,
		Query:        query.FrozenCopy(), // make a copy to ensure immutability
	})
	return id
}

func qualify(system, id string) string {
	return system + "_" + id
}

// AddChangelist returns a builder for data belonging to a changelist.
func (b *TablesBuilder) AddChangelist(id, crs, owner, subject string, status schema.ChangelistStatus) *ChangelistBuilder {
	if len(b.groupingKeys) == 0 {
		logAndPanic("Must add grouping keys before traces")
	}
	cb := &ChangelistBuilder{
		changelist: schema.ChangelistRow{
			ChangelistID: qualify(crs, id),
			System:       crs,
			Status:       status,
			OwnerEmail:   owner,
			Subject:      subject,
		},
		groupingKeys: b.groupingKeys,
	}
	b.changelistBuilders = append(b.changelistBuilders, cb)
	return cb
}

// Build should be called when all the data has been loaded in for a given setup. It will generate
// the SQL rows as represented in a schema.Tables. If any validation steps fail, it will panic.
func (b *TablesBuilder) Build() schema.Tables {
	if b.tileWidth == 0 {
		b.tileWidth = 100 // default
	}
	var tables schema.Tables
	commitsWithData := map[schema.CommitID]bool{}
	valuesAtHead := map[schema.MD5Hash]*schema.ValueAtHeadRow{}
	for _, traceBuilder := range b.traceBuilders {
		// Add unique rows from the tables gathered by tracebuilders.
		for _, opt := range traceBuilder.options {
			tables.Options = addOptionIfUnique(tables.Options, opt)
		}
		for _, g := range traceBuilder.groupings {
			tables.Groupings = addGroupingIfUnique(tables.Groupings, g)
		}
		for _, sf := range traceBuilder.sourceFiles {
			tables.SourceFiles = addSourceFileIfUnique(tables.SourceFiles, sf)
		}

		for _, t := range traceBuilder.traces {
			var matchesAnyIgnoreRule schema.NullableBool
			// prevent accidental duplicates on primary branch.
			tables.Traces, matchesAnyIgnoreRule = b.addTrace(tables.Traces, t, false)
			valuesAtHead[sql.AsMD5Hash(t.TraceID)] = &schema.ValueAtHeadRow{
				TraceID:              t.TraceID,
				GroupingID:           t.GroupingID,
				Corpus:               t.Corpus,
				Keys:                 t.Keys,
				MatchesAnyIgnoreRule: matchesAnyIgnoreRule,
			}
		}

		for _, xtv := range traceBuilder.traceValues {
			for _, tv := range xtv {
				if tv != nil {
					if tv.TraceID == nil || tv.GroupingID == nil {
						panic("Incomplete data - you must call Keys()")
					}
					if tv.OptionsID == nil {
						panic("Incomplete data - you must call Options*()")
					}
					if tv.SourceFileID == nil {
						panic("Incomplete data - you must call IngestedFrom()")
					}
					tables.TraceValues = append(tables.TraceValues, *tv)
					commitsWithData[tv.CommitID] = true
					vHead := valuesAtHead[sql.AsMD5Hash(tv.TraceID)]
					vHead.Digest = tv.Digest
					vHead.MostRecentCommitID = tv.CommitID
					vHead.OptionsID = tv.OptionsID
				}
			}
		}
	}
	tables.TiledTraceDigests = b.computeTiledTraceDigests()
	tables.PrimaryBranchParams = b.computePrimaryBranchParams()
	tables.Commits = b.commitBuilder.commits
	for i := range tables.Commits {
		cid := tables.Commits[i].CommitID
		if commitsWithData[cid] {
			tables.Commits[i].HasData = true
		}
	}
	exp := b.finalizeExpectations()
	for _, e := range exp {
		tables.Expectations = append(tables.Expectations, *e)
	}
	for _, eb := range b.expectationBuilders {
		tables.ExpectationRecords = append(tables.ExpectationRecords, *eb.record)
		tables.ExpectationDeltas = append(tables.ExpectationDeltas, eb.deltas...)
	}

	for _, cl := range b.changelistBuilders {
		for _, ps := range cl.patchsets {
			for _, opt := range ps.options {
				tables.Options = addOptionIfUnique(tables.Options, opt)
			}
			for _, g := range ps.groupings {
				tables.Groupings = addGroupingIfUnique(tables.Groupings, g)
			}
			for _, sf := range ps.sourceFiles {
				tables.SourceFiles = addSourceFileIfUnique(tables.SourceFiles, sf)
			}
			for _, t := range ps.traces {
				// duplicates allowed for different changelists.
				tables.Traces, _ = b.addTrace(tables.Traces, t, true)
			}
			for _, tj := range ps.tryjobs {
				tables.Tryjobs = append(tables.Tryjobs, tj)
				if cl.changelist.LastIngestedData.Before(tj.LastIngestedData) {
					cl.changelist.LastIngestedData = tj.LastIngestedData
				}
			}
			for _, xdp := range ps.dataPoints {
				for _, dp := range xdp {
					tables.SecondaryBranchValues = append(tables.SecondaryBranchValues, *dp)
				}
			}
			tables.Patchsets = append(tables.Patchsets, ps.patchset)
		}
		tables.Changelists = append(tables.Changelists, cl.changelist)
		for _, clExpBuilder := range cl.expectationBuilders {
			record := *clExpBuilder.record
			record.NumChanges = len(clExpBuilder.deltas)
			tables.ExpectationRecords = append(tables.ExpectationRecords, record)
			tables.ExpectationDeltas = append(tables.ExpectationDeltas, clExpBuilder.deltas...)
			for _, delta := range clExpBuilder.deltas {
				tables.SecondaryBranchExpectations = append(tables.SecondaryBranchExpectations, schema.SecondaryBranchExpectationRow{
					BranchName:          *record.BranchName,
					GroupingID:          delta.GroupingID,
					Digest:              delta.Digest,
					Label:               delta.LabelAfter,
					ExpectationRecordID: record.ExpectationRecordID,
				})
			}
		}
	}
	tables.SecondaryBranchParams = b.computeSecondaryBranchParams()

	tables.DiffMetrics = b.diffMetrics
	for _, atHead := range valuesAtHead {
		for _, exp := range tables.Expectations {
			if bytes.Equal(exp.GroupingID, atHead.GroupingID) && bytes.Equal(exp.Digest, atHead.Digest) {
				atHead.ExpectationRecordID = exp.ExpectationRecordID
				atHead.Label = exp.Label
				break
			}
		}
		tables.ValuesAtHead = append(tables.ValuesAtHead, *atHead)
	}
	tables.IgnoreRules = b.ignoreRules
	return tables
}

func addOptionIfUnique(existing []schema.OptionsRow, opt schema.OptionsRow) []schema.OptionsRow {
	for _, existingOpt := range existing {
		if bytes.Equal(opt.OptionsID, existingOpt.OptionsID) {
			return existing
		}
	}
	return append(existing, opt)
}

func addGroupingIfUnique(existing []schema.GroupingRow, g schema.GroupingRow) []schema.GroupingRow {
	for _, existingG := range existing {
		if bytes.Equal(g.GroupingID, existingG.GroupingID) {
			return existing
		}
	}
	return append(existing, g)
}

func addSourceFileIfUnique(existing []schema.SourceFileRow, sf schema.SourceFileRow) []schema.SourceFileRow {
	for _, existingSF := range existing {
		if bytes.Equal(sf.SourceFileID, existingSF.SourceFileID) {
			return existing
		}
	}
	return append(existing, sf)
}

func (b *TablesBuilder) addTrace(existing []schema.TraceRow, t schema.TraceRow, allowDuplicates bool) ([]schema.TraceRow, schema.NullableBool) {
	for _, existingT := range existing {
		if bytes.Equal(t.TraceID, existingT.TraceID) {
			if allowDuplicates {
				// When adding traces from patchsets, it is expected for there to be duplicate
				// traces because patchsets can build onto existing traces or make new ones.
				return existing, existingT.MatchesAnyIgnoreRule
			}
			// Having a duplicate trace means that there are duplicate TraceValues entries
			// and that is not intended.
			logAndPanic("Duplicate trace found: %v", t.Keys)
		}
	}
	ignored := false
	for _, ir := range b.ignoreRules {
		if ir.Query.MatchesParams(t.Keys) {
			ignored = true
			break
		}
	}
	matches := schema.NBNull
	if len(b.ignoreRules) > 0 {
		if ignored {
			matches = schema.NBTrue
		} else {
			matches = schema.NBFalse
		}
	}
	t.MatchesAnyIgnoreRule = matches
	return append(existing, t), matches
}

type tiledTraceDigest struct {
	startCommitID schema.CommitID
	traceID       schema.MD5Hash
	digest        schema.MD5Hash
}

func (b *TablesBuilder) computeTiledTraceDigests() []schema.TiledTraceDigestRow {
	seenRows := map[tiledTraceDigest]bool{}
	for _, builder := range b.traceBuilders {
		for _, xtv := range builder.traceValues {
			for _, tv := range xtv {
				if tv == nil {
					continue
				}
				tiledID := sql.ComputeTileStartID(tv.CommitID, b.tileWidth)
				seenRows[tiledTraceDigest{
					startCommitID: tiledID,
					traceID:       sql.AsMD5Hash(tv.TraceID),
					digest:        sql.AsMD5Hash(tv.Digest),
				}] = true
			}
		}
	}
	var rv []schema.TiledTraceDigestRow
	for row := range seenRows {
		tID := make(schema.TraceID, len(schema.MD5Hash{}))
		db := make(schema.DigestBytes, len(schema.MD5Hash{}))
		copy(tID, row.traceID[:])
		copy(db, row.digest[:])
		rv = append(rv, schema.TiledTraceDigestRow{
			StartCommitID: row.startCommitID,
			TraceID:       tID,
			Digest:        db,
		})
	}
	return rv
}

// computePrimaryBranchParams goes through all trace data and returns the PrimaryBranchParamRow
// with the appropriately tiled key/value pairs that showed up in the trace keys and params.
func (b *TablesBuilder) computePrimaryBranchParams() []schema.PrimaryBranchParamRow {
	seenRows := map[schema.PrimaryBranchParamRow]bool{}
	for _, builder := range b.traceBuilders {
		findTraceKeys := func(traceID schema.TraceID) paramtools.Params {
			for _, tr := range builder.traces {
				if bytes.Equal(tr.TraceID, traceID) {
					return tr.Keys.Copy() // Copy to ensure immutability
				}
			}
			logAndPanic("missing trace id %x", traceID)
			return nil
		}
		findOptions := func(optID schema.OptionsID) paramtools.Params {
			for _, opt := range builder.options {
				if bytes.Equal(opt.OptionsID, optID) {
					return opt.Keys.Copy() // Copy to ensure immutability
				}
			}
			logAndPanic("missing options id %x", optID)
			return nil
		}
		for _, xtv := range builder.traceValues {
			for _, tv := range xtv {
				if tv == nil {
					continue
				}
				tiledID := sql.ComputeTileStartID(tv.CommitID, b.tileWidth)
				keys := findTraceKeys(tv.TraceID)
				for k, v := range keys {
					seenRows[schema.PrimaryBranchParamRow{
						StartCommitID: tiledID,
						Key:           k,
						Value:         v,
					}] = true
				}
				options := findOptions(tv.OptionsID)
				for k, v := range options {
					seenRows[schema.PrimaryBranchParamRow{
						StartCommitID: tiledID,
						Key:           k,
						Value:         v,
					}] = true
				}
			}
		}
	}
	var rv []schema.PrimaryBranchParamRow
	for row := range seenRows {
		rv = append(rv, row)
	}
	return rv
}

// computeSecondaryBranchParams goes through every PS of every built CL and creates a row for each
// key/value pair from the traces and the options and returns it (without duplicates). This assumes
// that traces are only made on a patchset that has some data associated with it. It is simpler
// than computePrimaryBranchParams because we don't need to worry about tiling.
func (b *TablesBuilder) computeSecondaryBranchParams() []schema.SecondaryBranchParamRow {
	seenRows := map[schema.SecondaryBranchParamRow]bool{}
	for _, cl := range b.changelistBuilders {
		for _, ps := range cl.patchsets {
			for _, tr := range ps.traces {
				for k, v := range tr.Keys {
					seenRows[schema.SecondaryBranchParamRow{
						BranchName:  ps.patchset.ChangelistID,
						VersionName: ps.patchset.PatchsetID,
						Key:         k,
						Value:       v,
					}] = true
				}
			}
			for _, opt := range ps.options {
				for k, v := range opt.Keys {
					seenRows[schema.SecondaryBranchParamRow{
						BranchName:  ps.patchset.ChangelistID,
						VersionName: ps.patchset.PatchsetID,
						Key:         k,
						Value:       v,
					}] = true
				}
			}
		}
	}
	var rv []schema.SecondaryBranchParamRow
	for row := range seenRows {
		rv = append(rv, row)
	}
	return rv
}

// CommitBuilder has methods for easily building commit history. All methods are chainable.
type CommitBuilder struct {
	commits    []schema.CommitRow
	previousID int
}

// Append adds a commit whose ID is one higher than the previous commits ID. It panics if
// the commitTime is not formatted to RFC3339.
func (b *CommitBuilder) Append(author, subject, commitTime string) *CommitBuilder {
	commitID := b.previousID + 1
	gitHash := fmt.Sprintf("%04d", commitID)
	// A true githash is 40 hex characters, so we repeat the 4 digits of the commitID 10 times.
	gitHash = strings.Repeat(gitHash, 10)
	ct, err := time.Parse(time.RFC3339, commitTime)
	if err != nil {
		logAndPanic("Invalid time %q: %s", commitTime, err)
	}
	b.commits = append(b.commits, schema.CommitRow{
		CommitID:    schema.CommitID(commitID),
		GitHash:     gitHash,
		CommitTime:  ct,
		AuthorEmail: author,
		Subject:     subject,
		HasData:     false,
	})
	b.previousID = commitID
	return b
}

// TraceBuilder has methods for easily building trace data. All methods are chainable.
type TraceBuilder struct {
	// inputs needed upon creation
	commits         []schema.CommitRow
	commonKeys      paramtools.Params
	groupingKeys    []string
	symbolsToDigest map[rune]schema.DigestBytes

	// built as a result of the calling methods
	groupings   []schema.GroupingRow
	options     []schema.OptionsRow
	sourceFiles []schema.SourceFileRow
	traceValues [][]*schema.TraceValueRow // each row is one trace's data points
	traces      []schema.TraceRow
}

// History takes in a slice of strings, with each string representing the history of a trace. Each
// string must have a number of symbols equal to the length of the number of commits. A dash '-'
// means no data at that commit; any other symbol must match the previous call to SetDigests().
// If any data is invalid or missing, this method panics.
func (b *TraceBuilder) History(traceHistories ...string) *TraceBuilder {
	if len(b.traceValues) > 0 {
		logAndPanic("History must be called only once.")
	}
	// traceValues will have length len(commits) * numTraces after this is complete. Some entries
	// may be nil to represent "no data" and will be stripped out later.
	for _, th := range traceHistories {
		if len(th) != len(b.commits) {
			logAndPanic("history %q is of invalid length: expected %d", th, len(b.commits))
		}
		traceValues := make([]*schema.TraceValueRow, len(b.commits))
		for i, symbol := range th {
			if symbol == '-' {
				continue
			}
			digest, ok := b.symbolsToDigest[symbol]
			if !ok {
				logAndPanic("Unknown symbol in trace history %s", string(symbol))
			}
			traceValues[i] = &schema.TraceValueRow{
				CommitID: b.commits[i].CommitID,
				Digest:   digest,
			}
		}
		b.traceValues = append(b.traceValues, traceValues)
	}
	return b
}

// Keys specifies the params for each trace. It must be called after History() and the keys param
// must have the same number of elements that the call to History() had. The nth element here
// represents the nth trace history. This method panics if any trace would end up being identical
// or lacks the grouping data. This method panics if called with incorrect parameters or at the
// wrong time in building chain.
func (b *TraceBuilder) Keys(keys []paramtools.Params) *TraceBuilder {
	if len(b.traceValues) == 0 {
		logAndPanic("Keys must be called after history loaded")
	}
	if len(b.traces) > 0 {
		logAndPanic("Keys must only once")
	}
	if len(keys) != len(b.traceValues) {
		logAndPanic("Expected one set of keys for each trace")
	}
	// We now have enough data to make all the traces.
	seenTraces := map[string]bool{} // maps serialized keys to if we have seen them.
	for i, traceParams := range keys {
		traceParams.Add(b.commonKeys)
		grouping := make(paramtools.Params, len(keys))
		for _, gk := range b.groupingKeys {
			val, ok := traceParams[gk]
			if !ok {
				logAndPanic("Missing grouping key %q from %v", gk, traceParams)
			}
			grouping[gk] = val
		}
		_, groupingID := sql.SerializeMap(grouping)
		traceJSON, traceID := sql.SerializeMap(traceParams)
		if seenTraces[traceJSON] {
			logAndPanic("Found identical trace %s", traceJSON)
		}
		seenTraces[traceJSON] = true
		for _, tv := range b.traceValues[i] {
			if tv != nil {
				tv.GroupingID = groupingID
				tv.TraceID = traceID
				tv.Shard = sql.ComputeTraceValueShard(traceID)
			}
		}
		b.groupings = append(b.groupings, schema.GroupingRow{
			GroupingID: groupingID,
			Keys:       grouping,
		})
		b.traces = append(b.traces, schema.TraceRow{
			TraceID:              traceID,
			Corpus:               traceParams[types.CorpusField],
			GroupingID:           groupingID,
			Keys:                 traceParams.Copy(), // make a copy to ensure immutability
			MatchesAnyIgnoreRule: schema.NBNull,
		})
	}
	return b
}

// OptionsAll applies the given options for all data points provided in history.
func (b *TraceBuilder) OptionsAll(opts paramtools.Params) *TraceBuilder {
	xopts := make([]paramtools.Params, len(b.traceValues))
	for i := range xopts {
		xopts[i] = opts
	}
	return b.OptionsPerTrace(xopts)
}

// OptionsPerTrace applies the given optional params to the traces created in History. The number
// of options is expected to match the number of traces. It panics if called more than once or
// at the wrong time.
func (b *TraceBuilder) OptionsPerTrace(xopts []paramtools.Params) *TraceBuilder {
	if len(b.traceValues) == 0 {
		logAndPanic("Options* must be called after history loaded")
	}
	if len(b.options) > 0 {
		logAndPanic("Must call Options* only once")
	}
	if len(xopts) != len(b.traceValues) {
		logAndPanic("Must have one options per trace")
	}
	for i, opts := range xopts {
		_, optionsID := sql.SerializeMap(opts)
		b.options = append(b.options, schema.OptionsRow{
			OptionsID: optionsID,
			Keys:      opts.Copy(), // make a copy to ensure immutability
		})
		// apply it to every trace value in the ith trace
		for _, tv := range b.traceValues[i] {
			if tv == nil {
				continue
			}
			tv.OptionsID = optionsID
		}
	}
	return b
}

// IngestedFrom applies the given list of files and ingested times to the provided data.
// The number of filenames and ingestedDates is expected to match the number of commits; if no
// data is at that commit, it is ok to have both entries be empty string. It panics if any inputs
// are invalid.
func (b *TraceBuilder) IngestedFrom(filenames, ingestedDates []string) *TraceBuilder {
	if len(b.traceValues) == 0 {
		logAndPanic("IngestedFrom must be called after history")
	}
	if len(b.sourceFiles) > 0 {
		logAndPanic("Must call IngestedFrom only once")
	}
	if len(filenames) != len(b.commits) {
		logAndPanic("Expected %d files", len(b.commits))
	}
	if len(ingestedDates) != len(b.commits) {
		logAndPanic("Expected %d dates", len(b.commits))
	}

	for i := range filenames {
		name, ingestedDate := filenames[i], ingestedDates[i]
		if name == "" && ingestedDate == "" {
			continue // not used by any traces
		}
		if name == "" || ingestedDate == "" {
			logAndPanic("both name and date should be empty, if one is")
		}
		h := md5.Sum([]byte(name))
		sourceID := h[:]

		d, err := time.Parse(time.RFC3339, ingestedDate)
		if err != nil {
			logAndPanic("Invalid date format %q: %s", ingestedDate, err)
		}
		b.sourceFiles = append(b.sourceFiles, schema.SourceFileRow{
			SourceFileID: sourceID,
			SourceFile:   name,
			LastIngested: d,
		})
		// apply it to every ith tracevalue.
		for _, traceRows := range b.traceValues {
			if traceRows[i] != nil {
				traceRows[i].SourceFileID = sourceID
			}
		}
	}
	return b
}

type ExpectationsBuilder struct {
	currentGroupingID schema.GroupingID
	deltas            []schema.ExpectationDeltaRow
	groupingKeys      []string
	record            *schema.ExpectationRecordRow
}

func (b *ExpectationsBuilder) ExpectationsForGrouping(keys paramtools.Params) *ExpectationsBuilder {
	for _, key := range b.groupingKeys {
		if _, ok := keys[key]; !ok {
			logAndPanic("Grouping is missing key %q", key)
		}
	}
	_, b.currentGroupingID = sql.SerializeMap(keys)
	return b
}

// Positive marks the given digest as positive for the current grouping. It assumes that the
// previous triage state was untriaged (as this is quite common for test data).
func (b *ExpectationsBuilder) Positive(d types.Digest) *ExpectationsBuilder {
	db, err := sql.DigestToBytes(d)
	if err != nil {
		logAndPanic("Invalid digest %q: %s", d, err)
	}
	b.deltas = append(b.deltas, schema.ExpectationDeltaRow{
		ExpectationRecordID: b.record.ExpectationRecordID,
		GroupingID:          b.currentGroupingID,
		Digest:              db,
		LabelBefore:         schema.LabelUntriaged,
		LabelAfter:          schema.LabelPositive,
	})
	return b
}

// Negative marks the given digest as negative for the current grouping. It assumes that the
// previous triage state was untriaged (as this is quite common for test data).
func (b *ExpectationsBuilder) Negative(d types.Digest) *ExpectationsBuilder {
	db, err := sql.DigestToBytes(d)
	if err != nil {
		logAndPanic("Invalid digest %q: %s", d, err)
	}
	b.deltas = append(b.deltas, schema.ExpectationDeltaRow{
		ExpectationRecordID: b.record.ExpectationRecordID,
		GroupingID:          b.currentGroupingID,
		Digest:              db,
		LabelBefore:         schema.LabelUntriaged,
		LabelAfter:          schema.LabelNegative,
	})
	return b
}

func logAndPanic(msg string, args ...interface{}) {
	panic(fmt.Sprintf(msg, args...))
}

type ChangelistBuilder struct {
	changelist          schema.ChangelistRow
	expectationBuilders []*ExpectationsBuilder
	groupingKeys        []string
	patchsets           []*PatchsetBuilder
}

// AddPatchset returns a builder for data associated with the given patchset.
func (b *ChangelistBuilder) AddPatchset(psID, gitHash string, order int) *PatchsetBuilder {
	pb := &PatchsetBuilder{
		patchset: schema.PatchsetRow{
			PatchsetID:   qualify(b.changelist.System, psID),
			System:       b.changelist.System,
			ChangelistID: b.changelist.ChangelistID,
			Order:        order,
			GitHash:      gitHash,
		},
		groupingKeys: b.groupingKeys,
	}
	b.patchsets = append(b.patchsets, pb)
	return pb
}

// AddTriageEvent returns a builder for a series of triage events on the current CL. These
// events will be attributed to the given user and timestamp.
func (b *ChangelistBuilder) AddTriageEvent(user, triageTime string) *ExpectationsBuilder {
	if len(b.groupingKeys) == 0 {
		logAndPanic("Must add grouping keys before expectations")
	}
	ts, err := time.Parse(time.RFC3339, triageTime)
	if err != nil {
		logAndPanic("Invalid triage time %q: %s", triageTime, err)
	}
	eb := &ExpectationsBuilder{
		groupingKeys: b.groupingKeys,
		record: &schema.ExpectationRecordRow{
			ExpectationRecordID: uuid.New(),
			BranchName:          &b.changelist.ChangelistID,
			UserName:            user,
			TriageTime:          ts,
		},
	}
	b.expectationBuilders = append(b.expectationBuilders, eb)
	return eb
}

type PatchsetBuilder struct {
	commonKeys   paramtools.Params
	dataPoints   [][]*schema.SecondaryBranchValueRow
	groupingKeys []string
	groupings    []schema.GroupingRow
	options      []schema.OptionsRow
	patchset     schema.PatchsetRow
	sourceFiles  []schema.SourceFileRow
	traces       []schema.TraceRow
	tryjobs      []schema.TryjobRow
}

// DataWithCommonKeys sets it so the next calls to Digests will use these trace keys.
func (b *PatchsetBuilder) DataWithCommonKeys(keys paramtools.Params) *PatchsetBuilder {
	if b.commonKeys != nil {
		logAndPanic("Cannot call DataWithCommonKeys twice")
	}
	b.commonKeys = keys
	return b
}

// Digests adds some data to this patchset. Each digest represents a single data point on a
// single trace.
func (b *PatchsetBuilder) Digests(digests ...types.Digest) *PatchsetBuilder {
	if len(digests) == 0 {
		panic("Cannot add empty data")
	}
	newData := make([]*schema.SecondaryBranchValueRow, 0, len(digests))
	for _, d := range digests {
		db, err := sql.DigestToBytes(d)
		if err != nil {
			logAndPanic("Invalid digest %q: %s", d, err)
		}
		newData = append(newData, &schema.SecondaryBranchValueRow{
			BranchName:  b.patchset.ChangelistID,
			VersionName: b.patchset.PatchsetID,
			Digest:      db,
		})
	}
	b.dataPoints = append(b.dataPoints, newData)
	return b
}

// Keys applies the given keys to the previous set of data added with Digests(), including
// a previously set common keys. The length of keys should match the earlier call to Digests().
func (b *PatchsetBuilder) Keys(keys []paramtools.Params) *PatchsetBuilder {
	if len(b.dataPoints) == 0 {
		logAndPanic("Must call Digests before Keys()")
	}
	lastData := b.dataPoints[len(b.dataPoints)-1]
	if len(lastData) != len(keys) {
		logAndPanic("Expected %d keys", len(lastData))
	}
	for i, traceParams := range keys {
		traceParams.Add(b.commonKeys)
		grouping := make(paramtools.Params, len(keys))
		for _, gk := range b.groupingKeys {
			val, ok := traceParams[gk]
			if !ok {
				logAndPanic("Missing grouping key %q from %v", gk, traceParams)
			}
			grouping[gk] = val
		}
		_, groupingID := sql.SerializeMap(grouping)
		_, traceID := sql.SerializeMap(traceParams)
		lastData[i].GroupingID = groupingID
		lastData[i].TraceID = traceID
		b.groupings = append(b.groupings, schema.GroupingRow{
			GroupingID: groupingID,
			Keys:       grouping,
		})
		b.traces = append(b.traces, schema.TraceRow{
			TraceID:              traceID,
			Corpus:               traceParams[types.CorpusField],
			GroupingID:           groupingID,
			Keys:                 traceParams.Copy(), // make a copy to ensure immutability
			MatchesAnyIgnoreRule: schema.NBNull,
		})
	}
	return b
}

// OptionsAll applies the given options to the entire previous set of data added with Digests().
func (b *PatchsetBuilder) OptionsAll(opts paramtools.Params) *PatchsetBuilder {
	if len(b.dataPoints) == 0 {
		logAndPanic("OptionsAll must be called after history loaded")
	}
	lastData := b.dataPoints[len(b.dataPoints)-1]
	xopts := make([]paramtools.Params, len(lastData))
	for i := range xopts {
		xopts[i] = opts
	}
	return b.OptionsPerPoint(xopts)
}

// OptionsPerPoint applies the given options to the previous set of data added with Digests(). The
// length of keys should match the earlier call to Digests().
func (b *PatchsetBuilder) OptionsPerPoint(xopts []paramtools.Params) *PatchsetBuilder {
	if len(b.dataPoints) == 0 {
		logAndPanic("OptionsPerPoint must be called after history loaded")
	}
	lastData := b.dataPoints[len(b.dataPoints)-1]
	if len(lastData) != len(xopts) {
		logAndPanic("Expected %d options", len(lastData))
	}
	for i, opts := range xopts {
		_, optionsID := sql.SerializeMap(opts)
		b.options = append(b.options, schema.OptionsRow{
			OptionsID: optionsID,
			Keys:      opts.Copy(), // make a copy to ensure immutability
		})
		lastData[i].OptionsID = optionsID
	}
	return b
}

// FromTryjob assigns all data previously added with Digests() to be from the given tryjob.
func (b *PatchsetBuilder) FromTryjob(id, cis, name, file, ingestedTS string) *PatchsetBuilder {
	if len(b.dataPoints) == 0 {
		logAndPanic("FromTryjob must be called after history loaded")
	}
	updated, err := time.Parse(time.RFC3339, ingestedTS)
	if err != nil {
		logAndPanic("invalid timestamp %q: %s", ingestedTS, err)
	}
	qID := qualify(cis, id)
	b.tryjobs = append(b.tryjobs, schema.TryjobRow{
		TryjobID:         qID,
		System:           cis,
		ChangelistID:     b.patchset.ChangelistID,
		PatchsetID:       b.patchset.PatchsetID,
		DisplayName:      name,
		LastIngestedData: updated,
	})
	h := md5.Sum([]byte(file))
	sourceID := h[:]
	b.sourceFiles = append(b.sourceFiles, schema.SourceFileRow{
		SourceFileID: sourceID,
		SourceFile:   file,
		LastIngested: updated,
	})
	lastData := b.dataPoints[len(b.dataPoints)-1]
	for _, d := range lastData {
		d.TryjobID = qID
		d.SourceFileID = sourceID
	}
	return b
}
