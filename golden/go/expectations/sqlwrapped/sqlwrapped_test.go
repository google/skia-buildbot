package sqlwrapped

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/expectations/mocks"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/sqltest"
	"go.skia.org/infra/golden/go/types"
)

func TestAddChange_NoExistingExpectations_WrittenToSQLAndFirestore(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	groupingZero := schema.GroupingID{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}
	groupingOne := schema.GroupingID{0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1}
	existingData := schema.Tables{Groupings: []schema.GroupingRow{{
		GroupingID: groupingZero,
		Keys: paramtools.Params{
			types.CorpusField:     "my_corpus",
			types.PrimaryKeyField: "grouping_zero",
		}}, {
		GroupingID: groupingOne,
		Keys: paramtools.Params{
			types.CorpusField:     "my_corpus",
			types.PrimaryKeyField: "grouping_one",
		},
	}}}
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))

	toChange := []expectations.Delta{{
		Grouping: "grouping_zero",
		Digest:   "00000000000000000000000000000000",
		Label:    expectations.Negative,
	}, {
		Grouping: "grouping_one",
		Digest:   "11111111111111111111111111111111",
		Label:    expectations.Positive,
	}, {
		Grouping: "grouping_one",
		Digest:   "22222222222222222222222222222222",
		Label:    expectations.Untriaged,
	}, {
		Grouping: "grouping_one",
		Digest:   "33333333333333333333333333333333",
		Label:    expectations.Positive,
	}}
	const userID = "some user"
	fakeNow := time.Date(2021, time.March, 22, 21, 20, 19, 0, time.UTC)
	ctx = overwriteNow(ctx, fakeNow)

	ms := &mocks.Store{}
	ms.On("AddChange", testutils.AnyContext, toChange, userID).Return(nil)

	sw := &Impl{
		LegacyStore: ms,
		sqlDB:       db,
	}

	require.NoError(t, sw.AddChange(ctx, toChange, userID))
	ms.AssertExpectations(t)

	actualRecords := sqltest.GetAllRows(ctx, t, db, "ExpectationRecords", &schema.ExpectationRecordRow{}).([]schema.ExpectationRecordRow)
	require.Len(t, actualRecords, 1)
	expID := actualRecords[0].ExpectationRecordID
	assert.Equal(t, schema.ExpectationRecordRow{
		ExpectationRecordID: expID,
		BranchName:          nil,
		UserName:            userID,
		TriageTime:          fakeNow,
		NumChanges:          4,
	}, actualRecords[0])

	actualExpectations := sqltest.GetAllRows(ctx, t, db, "Expectations", &schema.ExpectationRow{}).([]schema.ExpectationRow)
	assert.Equal(t, []schema.ExpectationRow{{
		GroupingID:          groupingZero,
		Digest:              d("00000000000000000000000000000000"),
		Label:               schema.LabelNegative,
		ExpectationRecordID: &expID,
	}, {
		GroupingID:          groupingOne,
		Digest:              d("11111111111111111111111111111111"),
		Label:               schema.LabelPositive,
		ExpectationRecordID: &expID,
	}, {
		GroupingID:          groupingOne,
		Digest:              d("22222222222222222222222222222222"),
		Label:               schema.LabelUntriaged,
		ExpectationRecordID: &expID,
	}, {
		GroupingID:          groupingOne,
		Digest:              d("33333333333333333333333333333333"),
		Label:               schema.LabelPositive,
		ExpectationRecordID: &expID,
	}}, actualExpectations)

	actualDeltas := sqltest.GetAllRows(ctx, t, db, "ExpectationDeltas", &schema.ExpectationDeltaRow{}).([]schema.ExpectationDeltaRow)
	assert.Equal(t, []schema.ExpectationDeltaRow{{
		GroupingID:          groupingZero,
		Digest:              d("00000000000000000000000000000000"),
		LabelBefore:         schema.LabelUntriaged,
		LabelAfter:          schema.LabelNegative,
		ExpectationRecordID: expID,
	}, {
		GroupingID:          groupingOne,
		Digest:              d("11111111111111111111111111111111"),
		LabelBefore:         schema.LabelUntriaged,
		LabelAfter:          schema.LabelPositive,
		ExpectationRecordID: expID,
	}, {
		GroupingID:          groupingOne,
		Digest:              d("22222222222222222222222222222222"),
		LabelBefore:         schema.LabelUntriaged,
		LabelAfter:          schema.LabelUntriaged,
		ExpectationRecordID: expID,
	}, {
		GroupingID:          groupingOne,
		Digest:              d("33333333333333333333333333333333"),
		LabelBefore:         schema.LabelUntriaged,
		LabelAfter:          schema.LabelPositive,
		ExpectationRecordID: expID,
	}}, actualDeltas)

	actualSecondary := sqltest.GetAllRows(ctx, t, db, "SecondaryBranchExpectations", &schema.SecondaryBranchExpectationRow{}).([]schema.SecondaryBranchExpectationRow)
	assert.Empty(t, actualSecondary)
}

func TestAddChange_ExistingExpectations_WrittenToSQLAndFirestore(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	groupingZero := schema.GroupingID{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}
	groupingOne := schema.GroupingID{0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1}
	existingID := uuid.New() // should be overwritten for some values
	existingData := schema.Tables{
		Groupings: []schema.GroupingRow{{
			GroupingID: groupingZero,
			Keys: paramtools.Params{
				types.CorpusField:     "my_corpus",
				types.PrimaryKeyField: "grouping_zero",
			}}, {
			GroupingID: groupingOne,
			Keys: paramtools.Params{
				types.CorpusField:     "my_corpus",
				types.PrimaryKeyField: "grouping_one",
			},
		}},
		Expectations: []schema.ExpectationRow{{
			GroupingID:          groupingZero,
			Digest:              d("00000000000000000000000000000000"),
			Label:               schema.LabelPositive,
			ExpectationRecordID: &existingID,
		}, {
			GroupingID:          groupingOne,
			Digest:              d("22222222222222222222222222222222"),
			Label:               schema.LabelNegative,
			ExpectationRecordID: &existingID,
		}, {
			GroupingID:          groupingOne,
			Digest:              d("99999999999999999999999999999999"),
			Label:               schema.LabelNegative,
			ExpectationRecordID: &existingID,
		}}}
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))

	toChange := []expectations.Delta{{
		Grouping: "grouping_zero",
		Digest:   "00000000000000000000000000000000",
		Label:    expectations.Negative,
	}, {
		Grouping: "grouping_one",
		Digest:   "11111111111111111111111111111111",
		Label:    expectations.Positive,
	}, {
		Grouping: "grouping_one",
		Digest:   "22222222222222222222222222222222",
		Label:    expectations.Untriaged,
	}, {
		Grouping: "grouping_one",
		Digest:   "33333333333333333333333333333333",
		Label:    expectations.Positive,
	}}
	const userID = "some user"
	fakeNow := time.Date(2021, time.March, 22, 21, 20, 19, 0, time.UTC)
	ctx = overwriteNow(ctx, fakeNow)

	ms := &mocks.Store{}
	ms.On("AddChange", testutils.AnyContext, toChange, userID).Return(nil)

	sw := &Impl{
		LegacyStore: ms,
		sqlDB:       db,
	}

	require.NoError(t, sw.AddChange(ctx, toChange, userID))
	ms.AssertExpectations(t)

	actualRecords := sqltest.GetAllRows(ctx, t, db, "ExpectationRecords", &schema.ExpectationRecordRow{}).([]schema.ExpectationRecordRow)
	require.Len(t, actualRecords, 1)
	expID := actualRecords[0].ExpectationRecordID
	assert.Equal(t, schema.ExpectationRecordRow{
		ExpectationRecordID: expID,
		BranchName:          nil,
		UserName:            userID,
		TriageTime:          fakeNow,
		NumChanges:          4,
	}, actualRecords[0])

	actualExpectations := sqltest.GetAllRows(ctx, t, db, "Expectations", &schema.ExpectationRow{}).([]schema.ExpectationRow)
	assert.Equal(t, []schema.ExpectationRow{{
		GroupingID:          groupingZero,
		Digest:              d("00000000000000000000000000000000"),
		Label:               schema.LabelNegative,
		ExpectationRecordID: &expID,
	}, {
		GroupingID:          groupingOne,
		Digest:              d("11111111111111111111111111111111"),
		Label:               schema.LabelPositive,
		ExpectationRecordID: &expID,
	}, {
		GroupingID:          groupingOne,
		Digest:              d("22222222222222222222222222222222"),
		Label:               schema.LabelUntriaged,
		ExpectationRecordID: &expID,
	}, {
		GroupingID:          groupingOne,
		Digest:              d("33333333333333333333333333333333"),
		Label:               schema.LabelPositive,
		ExpectationRecordID: &expID,
	}, {
		GroupingID:          groupingOne,
		Digest:              d("99999999999999999999999999999999"),
		Label:               schema.LabelNegative,
		ExpectationRecordID: &existingID,
	}}, actualExpectations)

	actualDeltas := sqltest.GetAllRows(ctx, t, db, "ExpectationDeltas", &schema.ExpectationDeltaRow{}).([]schema.ExpectationDeltaRow)
	assert.Equal(t, []schema.ExpectationDeltaRow{{
		GroupingID:          groupingZero,
		Digest:              d("00000000000000000000000000000000"),
		LabelBefore:         schema.LabelPositive,
		LabelAfter:          schema.LabelNegative,
		ExpectationRecordID: expID,
	}, {
		GroupingID:          groupingOne,
		Digest:              d("11111111111111111111111111111111"),
		LabelBefore:         schema.LabelUntriaged,
		LabelAfter:          schema.LabelPositive,
		ExpectationRecordID: expID,
	}, {
		GroupingID:          groupingOne,
		Digest:              d("22222222222222222222222222222222"),
		LabelBefore:         schema.LabelNegative,
		LabelAfter:          schema.LabelUntriaged,
		ExpectationRecordID: expID,
	}, {
		GroupingID:          groupingOne,
		Digest:              d("33333333333333333333333333333333"),
		LabelBefore:         schema.LabelUntriaged,
		LabelAfter:          schema.LabelPositive,
		ExpectationRecordID: expID,
	}}, actualDeltas)
}

func TestAddChange_OneGroupingMissing_PartiallyWrittenToSQL(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	groupingZero := schema.GroupingID{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}
	existingData := schema.Tables{Groupings: []schema.GroupingRow{{
		GroupingID: groupingZero,
		Keys: paramtools.Params{
			types.CorpusField:     "my_corpus",
			types.PrimaryKeyField: "grouping_zero",
		}}}}
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))

	toChange := []expectations.Delta{{
		Grouping: "grouping_zero",
		Digest:   "00000000000000000000000000000000",
		Label:    expectations.Negative,
	}, {
		Grouping: "grouping_one",
		Digest:   "11111111111111111111111111111111",
		Label:    expectations.Positive,
	}, {
		Grouping: "grouping_one",
		Digest:   "22222222222222222222222222222222",
		Label:    expectations.Untriaged,
	}, {
		Grouping: "grouping_one",
		Digest:   "33333333333333333333333333333333",
		Label:    expectations.Positive,
	}}
	const userID = "some user"
	fakeNow := time.Date(2021, time.March, 22, 21, 20, 19, 0, time.UTC)
	ctx = overwriteNow(ctx, fakeNow)

	ms := &mocks.Store{}
	ms.On("AddChange", testutils.AnyContext, toChange, userID).Return(nil)

	sw := &Impl{
		LegacyStore: ms,
		sqlDB:       db,
	}

	require.NoError(t, sw.AddChange(ctx, toChange, userID))
	ms.AssertExpectations(t)

	actualRecords := sqltest.GetAllRows(ctx, t, db, "ExpectationRecords", &schema.ExpectationRecordRow{}).([]schema.ExpectationRecordRow)
	require.Len(t, actualRecords, 1)
	expID := actualRecords[0].ExpectationRecordID
	assert.Equal(t, schema.ExpectationRecordRow{
		ExpectationRecordID: expID,
		BranchName:          nil,
		UserName:            userID,
		TriageTime:          fakeNow,
		NumChanges:          1,
	}, actualRecords[0])

	actualExpectations := sqltest.GetAllRows(ctx, t, db, "Expectations", &schema.ExpectationRow{}).([]schema.ExpectationRow)
	assert.Equal(t, []schema.ExpectationRow{{
		GroupingID:          groupingZero,
		Digest:              d("00000000000000000000000000000000"),
		Label:               schema.LabelNegative,
		ExpectationRecordID: &expID,
	}}, actualExpectations)

	actualDeltas := sqltest.GetAllRows(ctx, t, db, "ExpectationDeltas", &schema.ExpectationDeltaRow{}).([]schema.ExpectationDeltaRow)
	assert.Equal(t, []schema.ExpectationDeltaRow{{
		GroupingID:          groupingZero,
		Digest:              d("00000000000000000000000000000000"),
		LabelBefore:         schema.LabelUntriaged,
		LabelAfter:          schema.LabelNegative,
		ExpectationRecordID: expID,
	}}, actualDeltas)
}

func TestAddChange_AllGroupingsMissing_NoDataWrittenToSQL(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)

	toChange := []expectations.Delta{{
		Grouping: "grouping_zero",
		Digest:   "00000000000000000000000000000000",
		Label:    expectations.Negative,
	}, {
		Grouping: "grouping_one",
		Digest:   "11111111111111111111111111111111",
		Label:    expectations.Positive,
	}, {
		Grouping: "grouping_one",
		Digest:   "22222222222222222222222222222222",
		Label:    expectations.Untriaged,
	}, {
		Grouping: "grouping_one",
		Digest:   "33333333333333333333333333333333",
		Label:    expectations.Positive,
	}}
	const userID = "some user"
	fakeNow := time.Date(2021, time.March, 22, 21, 20, 19, 0, time.UTC)
	ctx = overwriteNow(ctx, fakeNow)

	ms := &mocks.Store{}
	ms.On("AddChange", testutils.AnyContext, toChange, userID).Return(nil)

	sw := &Impl{
		LegacyStore: ms,
		sqlDB:       db,
	}

	require.NoError(t, sw.AddChange(ctx, toChange, userID))
	ms.AssertExpectations(t)

	actualRecords := sqltest.GetAllRows(ctx, t, db, "ExpectationRecords", &schema.ExpectationRecordRow{}).([]schema.ExpectationRecordRow)
	assert.Empty(t, actualRecords)

	actualExpectations := sqltest.GetAllRows(ctx, t, db, "Expectations", &schema.ExpectationRow{}).([]schema.ExpectationRow)
	assert.Empty(t, actualExpectations)

	actualDeltas := sqltest.GetAllRows(ctx, t, db, "ExpectationDeltas", &schema.ExpectationDeltaRow{}).([]schema.ExpectationDeltaRow)
	assert.Empty(t, actualDeltas)
}

func TestAddChange_FirestoreError_NothingWritten(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	groupingZero := schema.GroupingID{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}
	groupingOne := schema.GroupingID{0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1}
	existingData := schema.Tables{Groupings: []schema.GroupingRow{{
		GroupingID: groupingZero,
		Keys: paramtools.Params{
			types.CorpusField:     "my_corpus",
			types.PrimaryKeyField: "grouping_zero",
		}}, {
		GroupingID: groupingOne,
		Keys: paramtools.Params{
			types.CorpusField:     "my_corpus",
			types.PrimaryKeyField: "grouping_one",
		},
	}}}
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))

	toChange := []expectations.Delta{{
		Grouping: "grouping_zero",
		Digest:   "00000000000000000000000000000000",
		Label:    expectations.Negative,
	}, {
		Grouping: "grouping_one",
		Digest:   "11111111111111111111111111111111",
		Label:    expectations.Positive,
	}, {
		Grouping: "grouping_one",
		Digest:   "22222222222222222222222222222222",
		Label:    expectations.Untriaged,
	}, {
		Grouping: "grouping_one",
		Digest:   "33333333333333333333333333333333",
		Label:    expectations.Positive,
	}}
	const userID = "some user"
	fakeNow := time.Date(2021, time.March, 22, 21, 20, 19, 0, time.UTC)
	ctx = overwriteNow(ctx, fakeNow)

	ms := &mocks.Store{}
	ms.On("AddChange", testutils.AnyContext, toChange, userID).Return(errors.New("boom"))

	sw := &Impl{
		LegacyStore: ms,
		sqlDB:       db,
	}

	require.Error(t, sw.AddChange(ctx, toChange, userID))
	ms.AssertExpectations(t)

	actualRecords := sqltest.GetAllRows(ctx, t, db, "ExpectationRecords", &schema.ExpectationRecordRow{}).([]schema.ExpectationRecordRow)
	assert.Empty(t, actualRecords)

	actualExpectations := sqltest.GetAllRows(ctx, t, db, "Expectations", &schema.ExpectationRow{}).([]schema.ExpectationRow)
	assert.Empty(t, actualExpectations)

	actualDeltas := sqltest.GetAllRows(ctx, t, db, "ExpectationDeltas", &schema.ExpectationDeltaRow{}).([]schema.ExpectationDeltaRow)
	assert.Empty(t, actualDeltas)
}

func TestAddChange_SecondaryBranch_WrittenToSQLAndFirestore(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	groupingZero := schema.GroupingID{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}
	groupingOne := schema.GroupingID{0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1}
	existingData := schema.Tables{Groupings: []schema.GroupingRow{{
		GroupingID: groupingZero,
		Keys: paramtools.Params{
			types.CorpusField:     "my_corpus",
			types.PrimaryKeyField: "grouping_zero",
		}}, {
		GroupingID: groupingOne,
		Keys: paramtools.Params{
			types.CorpusField:     "my_corpus",
			types.PrimaryKeyField: "grouping_one",
		},
	}}}
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))

	toChange := []expectations.Delta{{
		Grouping: "grouping_zero",
		Digest:   "00000000000000000000000000000000",
		Label:    expectations.Negative,
	}, {
		Grouping: "grouping_one",
		Digest:   "11111111111111111111111111111111",
		Label:    expectations.Positive,
	}, {
		Grouping: "grouping_one",
		Digest:   "22222222222222222222222222222222",
		Label:    expectations.Untriaged,
	}, {
		Grouping: "grouping_one",
		Digest:   "33333333333333333333333333333333",
		Label:    expectations.Positive,
	}}
	const userID = "some user"
	branchName := "gerrit_1234567"
	fakeNow := time.Date(2021, time.March, 22, 21, 20, 19, 0, time.UTC)
	ctx = overwriteNow(ctx, fakeNow)

	ms := &mocks.Store{}
	ms.On("AddChange", testutils.AnyContext, toChange, userID).Return(nil)

	sw := &Impl{
		LegacyStore: ms,
		sqlDB:       db,
		branch:      "gerrit_1234567",
	}

	require.NoError(t, sw.AddChange(ctx, toChange, userID))
	ms.AssertExpectations(t)

	actualRecords := sqltest.GetAllRows(ctx, t, db, "ExpectationRecords", &schema.ExpectationRecordRow{}).([]schema.ExpectationRecordRow)
	require.Len(t, actualRecords, 1)
	expID := actualRecords[0].ExpectationRecordID
	assert.Equal(t, schema.ExpectationRecordRow{
		ExpectationRecordID: expID,
		BranchName:          &branchName,
		UserName:            userID,
		TriageTime:          fakeNow,
		NumChanges:          4,
	}, actualRecords[0])

	actualExpectations := sqltest.GetAllRows(ctx, t, db, "Expectations", &schema.ExpectationRow{}).([]schema.ExpectationRow)
	assert.Empty(t, actualExpectations)

	actualSecondary := sqltest.GetAllRows(ctx, t, db, "SecondaryBranchExpectations", &schema.SecondaryBranchExpectationRow{}).([]schema.SecondaryBranchExpectationRow)
	assert.Equal(t, []schema.SecondaryBranchExpectationRow{{
		GroupingID:          groupingZero,
		Digest:              d("00000000000000000000000000000000"),
		Label:               schema.LabelNegative,
		ExpectationRecordID: expID,
		BranchName:          branchName,
	}, {
		GroupingID:          groupingOne,
		Digest:              d("11111111111111111111111111111111"),
		Label:               schema.LabelPositive,
		ExpectationRecordID: expID,
		BranchName:          branchName,
	}, {
		GroupingID:          groupingOne,
		Digest:              d("22222222222222222222222222222222"),
		Label:               schema.LabelUntriaged,
		ExpectationRecordID: expID,
		BranchName:          branchName,
	}, {
		GroupingID:          groupingOne,
		Digest:              d("33333333333333333333333333333333"),
		Label:               schema.LabelPositive,
		ExpectationRecordID: expID,
		BranchName:          branchName,
	}}, actualSecondary)

	actualDeltas := sqltest.GetAllRows(ctx, t, db, "ExpectationDeltas", &schema.ExpectationDeltaRow{}).([]schema.ExpectationDeltaRow)
	assert.Equal(t, []schema.ExpectationDeltaRow{{
		GroupingID:          groupingZero,
		Digest:              d("00000000000000000000000000000000"),
		LabelBefore:         schema.LabelUntriaged,
		LabelAfter:          schema.LabelNegative,
		ExpectationRecordID: expID,
	}, {
		GroupingID:          groupingOne,
		Digest:              d("11111111111111111111111111111111"),
		LabelBefore:         schema.LabelUntriaged,
		LabelAfter:          schema.LabelPositive,
		ExpectationRecordID: expID,
	}, {
		GroupingID:          groupingOne,
		Digest:              d("22222222222222222222222222222222"),
		LabelBefore:         schema.LabelUntriaged,
		LabelAfter:          schema.LabelUntriaged,
		ExpectationRecordID: expID,
	}, {
		GroupingID:          groupingOne,
		Digest:              d("33333333333333333333333333333333"),
		LabelBefore:         schema.LabelUntriaged,
		LabelAfter:          schema.LabelPositive,
		ExpectationRecordID: expID,
	}}, actualDeltas)
}

func TestAddChange_SecondaryBranchWithExistingPrimaryBranch_WrittenToSQLAndFirestore(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	groupingZero := schema.GroupingID{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}
	groupingOne := schema.GroupingID{0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1}
	existingID := uuid.New() // should not be touched on primary branch
	existingData := schema.Tables{
		Groupings: []schema.GroupingRow{{
			GroupingID: groupingZero,
			Keys: paramtools.Params{
				types.CorpusField:     "my_corpus",
				types.PrimaryKeyField: "grouping_zero",
			}}, {
			GroupingID: groupingOne,
			Keys: paramtools.Params{
				types.CorpusField:     "my_corpus",
				types.PrimaryKeyField: "grouping_one",
			},
		}},
		Expectations: []schema.ExpectationRow{{
			GroupingID:          groupingZero,
			Digest:              d("00000000000000000000000000000000"),
			Label:               schema.LabelPositive,
			ExpectationRecordID: &existingID,
		}, {
			GroupingID:          groupingOne,
			Digest:              d("22222222222222222222222222222222"),
			Label:               schema.LabelNegative,
			ExpectationRecordID: &existingID,
		}, {
			GroupingID:          groupingOne,
			Digest:              d("99999999999999999999999999999999"),
			Label:               schema.LabelNegative,
			ExpectationRecordID: &existingID,
		}}}
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))

	toChange := []expectations.Delta{{
		Grouping: "grouping_zero",
		Digest:   "00000000000000000000000000000000",
		Label:    expectations.Negative,
	}, {
		Grouping: "grouping_one",
		Digest:   "11111111111111111111111111111111",
		Label:    expectations.Positive,
	}, {
		Grouping: "grouping_one",
		Digest:   "22222222222222222222222222222222",
		Label:    expectations.Untriaged,
	}, {
		Grouping: "grouping_one",
		Digest:   "33333333333333333333333333333333",
		Label:    expectations.Positive,
	}}
	const userID = "some user"
	branchName := "gerrit_1234567"
	fakeNow := time.Date(2021, time.March, 22, 21, 20, 19, 0, time.UTC)
	ctx = overwriteNow(ctx, fakeNow)

	ms := &mocks.Store{}
	ms.On("AddChange", testutils.AnyContext, toChange, userID).Return(nil)

	sw := &Impl{
		LegacyStore: ms,
		sqlDB:       db,
		branch:      "gerrit_1234567",
	}

	require.NoError(t, sw.AddChange(ctx, toChange, userID))
	ms.AssertExpectations(t)

	actualRecords := sqltest.GetAllRows(ctx, t, db, "ExpectationRecords", &schema.ExpectationRecordRow{}).([]schema.ExpectationRecordRow)
	require.Len(t, actualRecords, 1)
	expID := actualRecords[0].ExpectationRecordID
	assert.Equal(t, schema.ExpectationRecordRow{
		ExpectationRecordID: expID,
		BranchName:          &branchName,
		UserName:            userID,
		TriageTime:          fakeNow,
		NumChanges:          4,
	}, actualRecords[0])

	// These should be unchanged
	actualExpectations := sqltest.GetAllRows(ctx, t, db, "Expectations", &schema.ExpectationRow{}).([]schema.ExpectationRow)
	assert.Equal(t, []schema.ExpectationRow{{
		GroupingID:          groupingZero,
		Digest:              d("00000000000000000000000000000000"),
		Label:               schema.LabelPositive,
		ExpectationRecordID: &existingID,
	}, {
		GroupingID:          groupingOne,
		Digest:              d("22222222222222222222222222222222"),
		Label:               schema.LabelNegative,
		ExpectationRecordID: &existingID,
	}, {
		GroupingID:          groupingOne,
		Digest:              d("99999999999999999999999999999999"),
		Label:               schema.LabelNegative,
		ExpectationRecordID: &existingID,
	}}, actualExpectations)

	actualSecondary := sqltest.GetAllRows(ctx, t, db, "SecondaryBranchExpectations", &schema.SecondaryBranchExpectationRow{}).([]schema.SecondaryBranchExpectationRow)
	assert.Equal(t, []schema.SecondaryBranchExpectationRow{{
		GroupingID:          groupingZero,
		Digest:              d("00000000000000000000000000000000"),
		Label:               schema.LabelNegative,
		ExpectationRecordID: expID,
		BranchName:          branchName,
	}, {
		GroupingID:          groupingOne,
		Digest:              d("11111111111111111111111111111111"),
		Label:               schema.LabelPositive,
		ExpectationRecordID: expID,
		BranchName:          branchName,
	}, {
		GroupingID:          groupingOne,
		Digest:              d("22222222222222222222222222222222"),
		Label:               schema.LabelUntriaged,
		ExpectationRecordID: expID,
		BranchName:          branchName,
	}, {
		GroupingID:          groupingOne,
		Digest:              d("33333333333333333333333333333333"),
		Label:               schema.LabelPositive,
		ExpectationRecordID: expID,
		BranchName:          branchName,
	}}, actualSecondary)

	actualDeltas := sqltest.GetAllRows(ctx, t, db, "ExpectationDeltas", &schema.ExpectationDeltaRow{}).([]schema.ExpectationDeltaRow)
	assert.Equal(t, []schema.ExpectationDeltaRow{{
		GroupingID:          groupingZero,
		Digest:              d("00000000000000000000000000000000"),
		LabelBefore:         schema.LabelPositive, // read from primary branch
		LabelAfter:          schema.LabelNegative,
		ExpectationRecordID: expID,
	}, {
		GroupingID:          groupingOne,
		Digest:              d("11111111111111111111111111111111"),
		LabelBefore:         schema.LabelUntriaged,
		LabelAfter:          schema.LabelPositive,
		ExpectationRecordID: expID,
	}, {
		GroupingID:          groupingOne,
		Digest:              d("22222222222222222222222222222222"),
		LabelBefore:         schema.LabelNegative, // read from primary branch
		LabelAfter:          schema.LabelUntriaged,
		ExpectationRecordID: expID,
	}, {
		GroupingID:          groupingOne,
		Digest:              d("33333333333333333333333333333333"),
		LabelBefore:         schema.LabelUntriaged,
		LabelAfter:          schema.LabelPositive,
		ExpectationRecordID: expID,
	}}, actualDeltas)
}

func TestUndoChange_WritesRowForManualCleanup(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)

	const expIDToUndo = "need to undo"
	const userID = "user to undo"
	fakeNow := time.Date(2021, time.March, 22, 21, 20, 19, 0, time.UTC)
	ctx = overwriteNow(ctx, fakeNow)

	ms := &mocks.Store{}
	ms.On("UndoChange", testutils.AnyContext, expIDToUndo, userID).Return(nil)

	sw := &Impl{
		LegacyStore: ms,
		sqlDB:       db,
	}

	require.NoError(t, sw.UndoChange(ctx, expIDToUndo, userID))

	actualUndos := sqltest.GetAllRows(ctx, t, db, "DeprecatedExpectationUndos", &schema.DeprecatedExpectationUndoRow{}).([]schema.DeprecatedExpectationUndoRow)
	require.Len(t, actualUndos, 1)
	id := actualUndos[0].ID
	assert.Equal(t, []schema.DeprecatedExpectationUndoRow{{
		ID:            id,
		ExpectationID: expIDToUndo,
		UserID:        userID,
		TS:            fakeNow,
	}}, actualUndos)
}

func TestUndoChange_FirestoreError_NoSQLWrites(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)

	const expIDToUndo = "need to undo"
	const userID = "user to undo"
	fakeNow := time.Date(2021, time.March, 22, 21, 20, 19, 0, time.UTC)
	ctx = overwriteNow(ctx, fakeNow)

	ms := &mocks.Store{}
	ms.On("UndoChange", testutils.AnyContext, expIDToUndo, userID).Return(errors.New("boom"))

	sw := &Impl{
		LegacyStore: ms,
		sqlDB:       db,
	}

	require.Error(t, sw.UndoChange(ctx, expIDToUndo, userID))

	actualUndos := sqltest.GetAllRows(ctx, t, db, "DeprecatedExpectationUndos", &schema.DeprecatedExpectationUndoRow{}).([]schema.DeprecatedExpectationUndoRow)
	require.Empty(t, actualUndos)
}

// d returns the bytes associated with the hex-encoded digest string.
func d(digest types.Digest) []byte {
	if len(digest) != 2*md5.Size {
		panic("digest wrong length " + string(digest))
	}
	b, err := hex.DecodeString(string(digest))
	if err != nil {
		panic(err)
	}
	return b
}

func overwriteNow(ctx context.Context, ts time.Time) context.Context {
	return context.WithValue(ctx, now.ContextKey, ts)
}
