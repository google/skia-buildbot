package sqlculpritstore

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/perf/go/culprit"
	pb "go.skia.org/infra/perf/go/culprit/proto/v1"
	"go.skia.org/infra/perf/go/culprit/sqlculpritstore/schema"
	"go.skia.org/infra/perf/go/sql/sqltest"
)

func setUp(t *testing.T) (culprit.Store, pool.Pool) {
	db := sqltest.NewCockroachDBForTests(t, "culprits")
	store, err := New(db)
	require.NoError(t, err)
	return store, db
}

func TestGet_HappyPath_ReturnsCulprits(t *testing.T) {
	store, db := setUp(t)
	ctx := context.Background()
	id := uuid.NewString()
	existingCommit := schema.CulpritSchema{
		Id:              id,
		Host:            "chromium.googlesource.com",
		Project:         "chromium/src",
		Ref:             "refs/head/main",
		Revision:        "123",
		AnomalyGroupIDs: []string{"a1"},
		IssueIds:        []string{"b1"},
	}
	populateDb(t, ctx, db, existingCommit)

	actual, err := store.Get(ctx, []string{id})

	require.NoError(t, err)
	expected := []*pb.Culprit{{
		Commit: &pb.Commit{
			Host:     "chromium.googlesource.com",
			Project:  "chromium/src",
			Ref:      "refs/head/main",
			Revision: "123",
		},
		AnomalyGroupIds: []string{"a1"},
		IssueIds:        []string{"b1"},
	}}
	assert.ElementsMatch(t, actual, expected)
}

func TestUpsert_MissingAnomlayGroupId_ReturnErr(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()
	commit := &pb.Commit{
		Host:     "chromium.googlesource.com",
		Project:  "chromium/src",
		Ref:      "refs/head/main",
		Revision: "123",
	}
	commits := []*pb.Commit{commit}
	_, err := store.Upsert(ctx, "", commits)
	assert.Error(t, err)
}

func TestUpsert_DifferentHost_ReturnErr(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()
	commit1 := &pb.Commit{
		Host:     "chromium.googlesource.com",
		Project:  "chromium/src",
		Ref:      "refs/head/main",
		Revision: "123",
	}
	commit2 := &pb.Commit{
		Host:     "chromium1.googlesource.com",
		Project:  "chromium/src",
		Ref:      "refs/head/main",
		Revision: "456",
	}
	commits := []*pb.Commit{commit1, commit2}
	_, err := store.Upsert(ctx, "111", commits)
	assert.Error(t, err)
}

func TestUpsert_DifferentProject_ReturnErr(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()
	commit1 := &pb.Commit{
		Host:     "chromium.googlesource.com",
		Project:  "chromium/src",
		Ref:      "refs/head/main",
		Revision: "123",
	}
	commit2 := &pb.Commit{
		Host:     "chromium.googlesource.com",
		Project:  "chromium1/src",
		Ref:      "refs/head/main",
		Revision: "456",
	}
	commits := []*pb.Commit{commit1, commit2}
	_, err := store.Upsert(ctx, "111", commits)
	assert.Error(t, err)
}

func TestUpsert_DifferentRef_ReturnErr(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()
	commit1 := &pb.Commit{
		Host:     "chromium.googlesource.com",
		Project:  "chromium/src",
		Ref:      "refs/head/main",
		Revision: "123",
	}
	commit2 := &pb.Commit{
		Host:     "chromium.googlesource.com",
		Project:  "chromium/src",
		Ref:      "refs/head/main1",
		Revision: "456",
	}
	commits := []*pb.Commit{commit1, commit2}
	_, err := store.Upsert(ctx, "111", commits)
	assert.Error(t, err)
}

func TestUpsert_InsertNewCulprit_UpdateDbAndReturnNil(t *testing.T) {
	store, db := setUp(t)
	ctx := context.Background()
	commit := &pb.Commit{
		Host:     "chromium.googlesource.com",
		Project:  "chromium/src",
		Ref:      "refs/head/main",
		Revision: "123",
	}
	commits := []*pb.Commit{commit}
	_, err := store.Upsert(ctx, "111", commits)
	require.NoError(t, err)

	expected := []schema.CulpritSchema{{
		Host:            "chromium.googlesource.com",
		Project:         "chromium/src",
		Ref:             "refs/head/main",
		Revision:        "123",
		AnomalyGroupIDs: []string{"111"},
	}}
	actual := getCulpritsFromDb(t, ctx, db)
	assert.ElementsMatch(t, actual, expected)
}

func TestUpsert_UpdateExistingCulprit_UpdateDbAndReturnNil(t *testing.T) {
	store, db := setUp(t)
	ctx := context.Background()
	existingCommit := schema.CulpritSchema{
		Id:              uuid.NewString(),
		Host:            "chromium.googlesource.com",
		Project:         "chromium/src",
		Ref:             "refs/head/main",
		Revision:        "123",
		AnomalyGroupIDs: []string{"111"},
	}
	populateDb(t, ctx, db, existingCommit)
	commit := &pb.Commit{
		Host:     "chromium.googlesource.com",
		Project:  "chromium/src",
		Ref:      "refs/head/main",
		Revision: "123",
	}
	commits := []*pb.Commit{commit}
	_, err := store.Upsert(ctx, "222", commits) // anomlay_group_id is different
	require.NoError(t, err)

	expected := []schema.CulpritSchema{{
		Host:            "chromium.googlesource.com",
		Project:         "chromium/src",
		Ref:             "refs/head/main",
		Revision:        "123",
		AnomalyGroupIDs: []string{"222", "111"},
	}}
	actual := getCulpritsFromDb(t, ctx, db)
	assert.ElementsMatch(t, actual, expected)
}

func TestAddIssueId_NoIssueId_UpdateDbAndReturnNil(t *testing.T) {
	store, db := setUp(t)
	ctx := context.Background()
	existingCommit := schema.CulpritSchema{
		Id:              uuid.NewString(),
		Host:            "chromium.googlesource.com",
		Project:         "chromium/src",
		Ref:             "refs/head/main",
		Revision:        "123",
		AnomalyGroupIDs: []string{"111"},
	}
	populateDb(t, ctx, db, existingCommit)

	err := store.AddIssueId(ctx, existingCommit.Id, "bugid")
	require.NoError(t, err)

	actual := getCulpritsFromDb(t, ctx, db)
	assert.Equal(t, len(actual), 1)
	assert.ElementsMatch(t, actual[0].IssueIds, []string{"bugid"})
}

func TestAddIssueId_ExistingIssueId_UpdateDbAndReturnNil(t *testing.T) {
	store, db := setUp(t)
	ctx := context.Background()
	existingCommit := schema.CulpritSchema{
		Id:              uuid.NewString(),
		Host:            "chromium.googlesource.com",
		Project:         "chromium/src",
		Ref:             "refs/head/main",
		Revision:        "123",
		AnomalyGroupIDs: []string{"111"},
		IssueIds:        []string{"bugid1"},
	}
	populateDb(t, ctx, db, existingCommit)

	err := store.AddIssueId(ctx, existingCommit.Id, "bugid2")
	require.NoError(t, err)

	actual := getCulpritsFromDb(t, ctx, db)
	assert.Equal(t, len(actual), 1)
	assert.ElementsMatch(t, actual[0].IssueIds, []string{"bugid1", "bugid2"})
}

func populateDb(t *testing.T, ctx context.Context, db pool.Pool, culprit schema.CulpritSchema) {
	const query = `INSERT INTO Culprits
		(id, host, project, ref, revision, anomaly_group_ids, issue_ids)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`
	if _, err := db.Exec(ctx, query, culprit.Id, culprit.Host, culprit.Project, culprit.Ref, culprit.Revision, culprit.AnomalyGroupIDs, culprit.IssueIds); err != nil {
		require.NoError(t, err)
	}
}

func getCulpritsFromDb(t *testing.T, ctx context.Context, db pool.Pool) []schema.CulpritSchema {
	actual := []schema.CulpritSchema{}
	rows, _ := db.Query(ctx, "SELECT host, project, ref, revision, anomaly_group_ids, issue_ids FROM Culprits")
	for rows.Next() {
		var culpritInDb schema.CulpritSchema
		if err := rows.Scan(&culpritInDb.Host, &culpritInDb.Project, &culpritInDb.Ref, &culpritInDb.Revision, &culpritInDb.AnomalyGroupIDs, &culpritInDb.IssueIds); err != nil {
			require.NoError(t, err)
		}
		actual = append(actual, culpritInDb)

	}
	return actual
}
