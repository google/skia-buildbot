package sqlculpritstore

import (
	"context"
	"encoding/json"
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
	db := sqltest.NewSpannerDBForTests(t, "culprits")
	store, err := New(db)
	require.NoError(t, err)
	return store, db
}

func TestGet_HappyPath_ReturnsCulprits(t *testing.T) {
	store, db := setUp(t)
	ctx := context.Background()
	id := uuid.NewString()
	groupIssueMap := map[string]string{"a1": "b1"}
	existingCommit := schema.CulpritSchema{
		Id:              id,
		Host:            "chromium.googlesource.com",
		Project:         "chromium/src",
		Ref:             "refs/head/main",
		Revision:        "123",
		AnomalyGroupIDs: []string{"a1"},
		IssueIds:        []string{"b1"},
		GroupIssueMap:   groupIssueMap,
	}
	populateDb(t, ctx, db, existingCommit)

	actual, err := store.Get(ctx, []string{id})

	require.NoError(t, err)
	expected := []*pb.Culprit{{
		Id: id,
		Commit: &pb.Commit{
			Host:     "chromium.googlesource.com",
			Project:  "chromium/src",
			Ref:      "refs/head/main",
			Revision: "123",
		},
		AnomalyGroupIds: []string{"a1"},
		IssueIds:        []string{"b1"},
		GroupIssueMap:   groupIssueMap,
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
		GroupIssueMap:   map[string]string(nil),
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
		GroupIssueMap:   map[string]string(nil),
	}}
	actual := getCulpritsFromDb(t, ctx, db)
	assert.ElementsMatch(t, actual, expected)
}

func TestAddIssueId_NoIssueId_UpdateDbAndReturnNil(t *testing.T) {
	store, db := setUp(t)
	ctx := context.Background()
	culpritID := uuid.NewString()
	existingCommit := schema.CulpritSchema{
		Id:              culpritID,
		Host:            "chromium.googlesource.com",
		Project:         "chromium/src",
		Ref:             "refs/head/main",
		Revision:        "123",
		AnomalyGroupIDs: []string{"111"},
		GroupIssueMap:   map[string]string{},
	}
	populateDb(t, ctx, db, existingCommit)

	err := store.AddIssueId(ctx, existingCommit.Id, "bugid", "111")
	require.NoError(t, err)

	actual, err := store.Get(ctx, []string{culpritID})
	assert.NoError(t, err)
	assert.Equal(t, len(actual), 1)
	assert.ElementsMatch(t, actual[0].IssueIds, []string{"bugid"})
	assert.Equal(t, actual[0].GroupIssueMap["111"], "bugid")
}

// This reproduce the error when the group issue map is NULL in db.
// Calling rows.Scan(... &group_issue_map_in_jsonb) will fail when group_issue_map_in_jsonb is a string.
// The error message:
//
//	Failed to read Culprit results: can't scan into dest[7]: cannot scan null into *string.
func TestGet_UpsertWithNoGroupIssueMap(t *testing.T) {
	store, db := setUp(t)
	ctx := context.Background()

	// We will have group issue map as NULL in the new culprit
	const query = `INSERT INTO Culprits
		(id, host, project, ref, revision, anomaly_group_ids)
		VALUES ($1,$2,$3,$4,$5,$6)`
	culpritId := uuid.NewString()
	if _, err := db.Exec(ctx, query, culpritId, "host", "project", "ref", "rev", []string{"gid1", "gid2"}); err != nil {
		require.NoError(t, err)
	}

	// Getting NULL vallue should not break.
	actual, err := store.Get(ctx, []string{culpritId})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(actual), "wrong result size")
	assert.Equal(t, "host", actual[0].Commit.Host, "wrong host")
	assert.Equal(t, map[string]string(nil), actual[0].GroupIssueMap, "wrong group issue map")
}

func TestAddIssueId_UnexpectedGroup_ReturnErr(t *testing.T) {
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

	err := store.AddIssueId(ctx, existingCommit.Id, "bugid2", "222")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not related to the culprit")
}

func TestAddIssueId_ExistingGroupAndIssue_ReturnErr(t *testing.T) {
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
		GroupIssueMap:   map[string]string{"111": "bugid1"},
	}
	populateDb(t, ctx, db, existingCommit)

	err := store.AddIssueId(ctx, existingCommit.Id, "bugid2", "111")
	require.Error(t, err)
	require.Contains(t, err.Error(), "group id 111 has related issue already")
}

func populateDb(t *testing.T, ctx context.Context, db pool.Pool, culprit schema.CulpritSchema) {
	group_issue_map, _ := json.Marshal(culprit.GroupIssueMap)
	const query = `INSERT INTO Culprits
		(id, host, project, ref, revision, anomaly_group_ids, issue_ids, group_issue_map)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`
	if _, err := db.Exec(ctx, query, culprit.Id, culprit.Host, culprit.Project, culprit.Ref, culprit.Revision, culprit.AnomalyGroupIDs, culprit.IssueIds, group_issue_map); err != nil {
		require.NoError(t, err)
	}
}

func getCulpritsFromDb(t *testing.T, ctx context.Context, db pool.Pool) []schema.CulpritSchema {
	actual := []schema.CulpritSchema{}
	rows, _ := db.Query(ctx, "SELECT host, project, ref, revision, anomaly_group_ids, issue_ids, group_issue_map FROM Culprits")
	for rows.Next() {
		var culpritInDb schema.CulpritSchema
		var group_issue_map_in_jsonb []byte
		if err := rows.Scan(&culpritInDb.Host, &culpritInDb.Project, &culpritInDb.Ref, &culpritInDb.Revision, &culpritInDb.AnomalyGroupIDs, &culpritInDb.IssueIds, &group_issue_map_in_jsonb); err != nil {
			require.NoError(t, err)
		}
		var group_issue_map map[string]string
		_ = json.Unmarshal(group_issue_map_in_jsonb, &group_issue_map)
		culpritInDb.GroupIssueMap = group_issue_map
		actual = append(actual, culpritInDb)

	}
	return actual
}

func TestGetAnomalyGroupIdsForIssueId_Success(t *testing.T) {
	store, db := setUp(t)
	ctx := context.Background()
	populateDb(t, ctx, db, schema.CulpritSchema{
		Id:              uuid.NewString(),
		Host:            "chromium.googlesource.com",
		Project:         "chromium/src",
		Ref:             "refs/head/main",
		Revision:        "123",
		AnomalyGroupIDs: []string{"ag1", "ag2"},
		IssueIds:        []string{"issue1"},
	})
	populateDb(t, ctx, db, schema.CulpritSchema{
		Id:              uuid.NewString(),
		Host:            "chromium.googlesource.com",
		Project:         "chromium/src",
		Ref:             "refs/head/main",
		Revision:        "456",
		AnomalyGroupIDs: []string{"ag3", "ag4"},
		IssueIds:        []string{"issue1"},
	})
	populateDb(t, ctx, db, schema.CulpritSchema{
		Id:              uuid.NewString(),
		Host:            "chromium.googlesource.com",
		Project:         "chromium/src",
		Ref:             "refs/head/main",
		Revision:        "789",
		AnomalyGroupIDs: []string{"ag5", "ag6"},
		IssueIds:        []string{"issue2"},
	})

	res, err := store.GetAnomalyGroupIdsForIssueId(ctx, "issue1")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"ag1", "ag2", "ag3", "ag4"}, res)
}

func TestGetAnomalyGroupIdsForIssueId_NonExistentIssueId(t *testing.T) {
	store, db := setUp(t)
	ctx := context.Background()
	populateDb(t, ctx, db, schema.CulpritSchema{
		Id:              uuid.NewString(),
		Host:            "chromium.googlesource.com",
		Project:         "chromium/src",
		Ref:             "refs/head/main",
		Revision:        "123",
		AnomalyGroupIDs: []string{"ag1", "ag2"},
		IssueIds:        []string{"issue1"},
	})

	res, err := store.GetAnomalyGroupIdsForIssueId(ctx, "non-existent-issue")
	require.NoError(t, err)
	assert.Empty(t, res)
}

func TestGetAnomalyGroupIdsForIssueId_NoAssociatedGroups(t *testing.T) {
	store, db := setUp(t)
	ctx := context.Background()
	populateDb(t, ctx, db, schema.CulpritSchema{
		Id:              uuid.NewString(),
		Host:            "chromium.googlesource.com",
		Project:         "chromium/src",
		Ref:             "refs/head/main",
		Revision:        "123",
		AnomalyGroupIDs: nil,
		IssueIds:        []string{"issue1"},
	})

	res, err := store.GetAnomalyGroupIdsForIssueId(ctx, "issue1")
	require.NoError(t, err)
	assert.Empty(t, res)
}

func TestGetAnomalyGroupIdsForIssueId_EmptyIssueId(t *testing.T) {
	store, db := setUp(t)
	ctx := context.Background()
	populateDb(t, ctx, db, schema.CulpritSchema{
		Id:              uuid.NewString(),
		Host:            "chromium.googlesource.com",
		Project:         "chromium/src",
		Ref:             "refs/head/main",
		Revision:        "123",
		AnomalyGroupIDs: []string{"ag1", "ag2"},
		IssueIds:        []string{"issue1"},
	})

	res, err := store.GetAnomalyGroupIdsForIssueId(ctx, "")
	require.NoError(t, err)
	assert.Empty(t, res)
}
