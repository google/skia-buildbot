package sqlculpritstore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/perf/go/culprit"
	pb "go.skia.org/infra/perf/go/culprit/proto"
	"go.skia.org/infra/perf/go/culprit/sqlculpritstore/schema"
	"go.skia.org/infra/perf/go/sql/sqltest"
)

func setUp(t *testing.T) (culprit.Store, pool.Pool) {
	db := sqltest.NewCockroachDBForTests(t, "culprits")
	store, err := New(db)
	require.NoError(t, err)
	return store, db
}

func TestUpsert_MissingAnomlayGroupId_ReturnErr(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()
	culprit := &pb.Culprit{
		Commit: &pb.Commit{
			Host:     "chromium.googlesource.com",
			Project:  "chromium/src",
			Ref:      "refs/head/main",
			Revision: "123",
		},
	}
	culprits := []*pb.Culprit{culprit}
	err := store.Upsert(ctx, "", culprits)
	assert.Error(t, err)
}

func TestUpsert_DifferentHost_ReturnErr(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()
	culprit1 := &pb.Culprit{
		Commit: &pb.Commit{
			Host:     "chromium.googlesource.com",
			Project:  "chromium/src",
			Ref:      "refs/head/main",
			Revision: "123",
		},
	}
	culprit2 := &pb.Culprit{
		Commit: &pb.Commit{
			Host:     "chromium1.googlesource.com",
			Project:  "chromium/src",
			Ref:      "refs/head/main",
			Revision: "456",
		},
	}
	culprits := []*pb.Culprit{culprit1, culprit2}
	err := store.Upsert(ctx, "111", culprits)
	assert.Error(t, err)
}

func TestUpsert_DifferentProject_ReturnErr(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()
	culprit1 := &pb.Culprit{
		Commit: &pb.Commit{
			Host:     "chromium.googlesource.com",
			Project:  "chromium/src",
			Ref:      "refs/head/main",
			Revision: "123",
		},
	}
	culprit2 := &pb.Culprit{
		Commit: &pb.Commit{
			Host:     "chromium.googlesource.com",
			Project:  "chromium1/src",
			Ref:      "refs/head/main",
			Revision: "456",
		},
	}
	culprits := []*pb.Culprit{culprit1, culprit2}
	err := store.Upsert(ctx, "111", culprits)
	assert.Error(t, err)
}

func TestUpsert_DifferentRef_ReturnErr(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()
	culprit1 := &pb.Culprit{
		Commit: &pb.Commit{
			Host:     "chromium.googlesource.com",
			Project:  "chromium/src",
			Ref:      "refs/head/main",
			Revision: "123",
		},
	}
	culprit2 := &pb.Culprit{
		Commit: &pb.Commit{
			Host:     "chromium.googlesource.com",
			Project:  "chromium/src",
			Ref:      "refs/head/main1",
			Revision: "456",
		},
	}
	culprits := []*pb.Culprit{culprit1, culprit2}
	err := store.Upsert(ctx, "111", culprits)
	assert.Error(t, err)
}

func TestUpsert_InsertNewCulprit_UpdateDbAndReturnNil(t *testing.T) {
	store, db := setUp(t)
	ctx := context.Background()
	culprit := &pb.Culprit{
		Commit: &pb.Commit{
			Host:     "chromium.googlesource.com",
			Project:  "chromium/src",
			Ref:      "refs/head/main",
			Revision: "123",
		},
	}
	culprits := []*pb.Culprit{culprit}
	err := store.Upsert(ctx, "111", culprits)
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
	existingCulprit := schema.CulpritSchema{
		Host:            "chromium.googlesource.com",
		Project:         "chromium/src",
		Ref:             "refs/head/main",
		Revision:        "123",
		AnomalyGroupIDs: []string{"111"},
		IssueIds:        []int64{111},
	}
	populateDb(t, ctx, db, existingCulprit)
	culprit := &pb.Culprit{
		Commit: &pb.Commit{
			Host:     "chromium.googlesource.com",
			Project:  "chromium/src",
			Ref:      "refs/head/main",
			Revision: "123",
		},
	}
	culprits := []*pb.Culprit{culprit}
	err := store.Upsert(ctx, "222", culprits) // anomlay_group_id is different
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

func populateDb(t *testing.T, ctx context.Context, db pool.Pool, culprit schema.CulpritSchema) {
	const query = `INSERT INTO Culprits
		(host, project, ref, revision, anomaly_group_ids, issue_ids)
		VALUES ($1,$2,$3,$4,$5,$6)`
	if _, err := db.Exec(ctx, query, culprit.Host, culprit.Project, culprit.Ref, culprit.Revision, culprit.AnomalyGroupIDs, culprit.IssueIds); err != nil {
		require.NoError(t, err)
	}
}

func getCulpritsFromDb(t *testing.T, ctx context.Context, db pool.Pool) []schema.CulpritSchema {
	actual := []schema.CulpritSchema{}
	rows, _ := db.Query(ctx, "SELECT host, project, ref, revision, anomaly_group_ids FROM Culprits")
	for rows.Next() {
		var culpritInDb schema.CulpritSchema
		if err := rows.Scan(&culpritInDb.Host, &culpritInDb.Project, &culpritInDb.Ref, &culpritInDb.Revision, &culpritInDb.AnomalyGroupIDs); err != nil {
			require.NoError(t, err)
		}
		actual = append(actual, culpritInDb)

	}
	return actual
}
