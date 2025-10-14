package sqlanomalygroupstore

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/perf/go/anomalygroup"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/sql/sqltest"
)

func setUp(t *testing.T) (anomalygroup.Store, pool.Pool) {
	db := sqltest.NewSpannerDBForTests(t, "anomalygroups")
	store, err := New(db, config.SpannerDataStoreType)
	require.NoError(t, err)
	return store, db
}

func TestCreate(t *testing.T) {
	store, db := setUp(t)
	ctx := context.Background()

	new_group_id, err := store.Create(ctx, "sub", "rev-abc", "domain-a", "benchmark-a", 100, 200, "REPORT")
	require.NoError(t, err)
	assert.NotEmpty(t, new_group_id)

	count_cmd := "SELECT COUNT(*) FROM AnomalyGroups"
	count := 0
	err = db.QueryRow(ctx, count_cmd).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestCreate_EmptyStrings(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()

	_, err := store.Create(ctx, "", "rev-abc", "domain-a", "benchmark-a", 100, 200, "REPORT")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty strings")
}

func TestCreate_InvalidCommits(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()

	_, err := store.Create(ctx, "sub", "rev-abc", "domain-a", "benchmark-a", 300, 200, "REPORT")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "smaller than the start")
}

func TestCreate_NegativeCommits(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()

	_, err := store.Create(ctx, "sub", "rev-abc", "domain-a", "benchmark-a", -100, 200, "REPORT")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "negative commit")
}

func TestLoadByID(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()

	new_group_id, err := store.Create(ctx, "sub", "rev-abc", "domain-a", "benchmark-a", 100, 200, "REPORT")
	require.NoError(t, err)
	assert.NotEmpty(t, new_group_id)

	group, err2 := store.LoadById(ctx, new_group_id)
	require.NoError(t, err2)
	assert.Equal(t, "REPORT", group.GroupAction.String())
	assert.Equal(t, "sub", group.SubsciptionName)
	assert.Equal(t, "rev-abc", group.SubscriptionRevision)
	assert.Equal(t, "benchmark-a", group.BenchmarkName)
}

func TestLoadByID_BadID(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()

	new_group_id, err := store.Create(ctx, "sub", "rev-abc", "domain-a", "benchmark-a", 100, 200, "REPORT")
	require.NoError(t, err)
	assert.NotEmpty(t, new_group_id)

	bad_id := new_group_id[:len(new_group_id)-1]
	_, err = store.LoadById(ctx, bad_id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid UUID")
}

func TestLoadByID_NoRow(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()

	new_group_id, err := store.Create(ctx, "sub", "rev-abc", "domain-a", "benchmark-a", 100, 200, "REPORT")
	require.NoError(t, err)
	assert.NotEmpty(t, new_group_id)

	_, err = store.LoadById(ctx, uuid.NewString())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no rows")
}

func TestFindGroup(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()

	_, err := store.Create(ctx, "sub", "rev-abc", "domain-a", "benchmark-a", 100, 200, "REPORT")
	require.NoError(t, err)
	_, err = store.Create(ctx, "sub", "rev-abc", "domain-a", "benchmark-a", 200, 300, "REPORT")
	require.NoError(t, err)
	_, err = store.Create(ctx, "sub", "rev-abc", "domain-b", "benchmark-a", 200, 300, "REPORT")
	require.NoError(t, err)

	groups, err2 := store.FindExistingGroup(ctx, "sub", "rev-abc", "domain-a", "benchmark-a", 100, 300, "REPORT")
	require.NoError(t, err2)
	assert.Equal(t, 2, len(groups))
}

func TestUpdateBisectID(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()

	new_group_id, err := store.Create(ctx, "sub", "rev-abc", "domain-a", "benchmark-a", 100, 200, "REPORT")
	require.NoError(t, err)
	assert.NotEmpty(t, new_group_id)

	err = store.UpdateBisectID(ctx, new_group_id,
		"3cb85993-d0a8-452e-86ec-cb5154aada9c")
	require.NoError(t, err)

	group, err2 := store.LoadById(ctx, new_group_id)
	require.NoError(t, err2)
	assert.Equal(t, "REPORT", group.GroupAction.String())
}

func TestUpdateBisectID_InvalidID(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()

	new_group_id, err := store.Create(ctx, "sub", "rev-abc", "domain-a", "benchmark-a", 100, 200, "REPORT")
	require.NoError(t, err)
	assert.NotEmpty(t, new_group_id)

	err = store.UpdateBisectID(ctx, new_group_id,
		"3cb85993-d0a8-452e-86ec-cb5154aada=")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid UUID value")
}

func TestUpdateReportedIssueID(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()

	new_group_id, err := store.Create(ctx, "sub", "rev-abc", "domain-a", "benchmark-a", 100, 200, "REPORT")
	require.NoError(t, err)
	assert.NotEmpty(t, new_group_id)

	err = store.UpdateReportedIssueID(ctx, new_group_id,
		"24fa5591-946b-44e4-bf09-3fd271588ee5")
	require.NoError(t, err)

	group, err2 := store.LoadById(ctx, new_group_id)
	require.NoError(t, err2)
	assert.Equal(t, "REPORT", group.GroupAction.String())
}

func TestUpdateReportedIssueID_InvalidID(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()

	new_group_id, err := store.Create(ctx, "sub", "rev-abc", "domain-a", "benchmark-a", 100, 200, "REPORT")
	require.NoError(t, err)
	assert.NotEmpty(t, new_group_id)

	err = store.UpdateReportedIssueID(ctx, new_group_id,
		"24fa5591-946b-44e4-bf09-3fd271588e=")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid UUID value")
}

func TestAddAnomalyID(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()

	new_group_id, err := store.Create(ctx, "sub", "rev-abc", "domain-a", "benchmark-a", 100, 200, "REPORT")
	require.NoError(t, err)
	assert.NotEmpty(t, new_group_id)

	err = store.AddAnomalyID(ctx, new_group_id,
		"b1fb4036-1883-4d9e-85d4-ed607629017a")
	require.NoError(t, err)
	err = store.AddAnomalyID(ctx, new_group_id,
		"a60414c6-2495-4ef7-834a-829b1a929100")
	require.NoError(t, err)

	group, err2 := store.LoadById(ctx, new_group_id)
	require.NoError(t, err2)
	assert.Equal(t, []string{
		"b1fb4036-1883-4d9e-85d4-ed607629017a",
		"a60414c6-2495-4ef7-834a-829b1a929100"}, group.AnomalyIds)
}

func TestAddAnomalyID_InvalidID(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()

	new_group_id, err := store.Create(ctx, "sub", "rev-abc", "domain-a", "benchmark-a", 100, 200, "REPORT")
	require.NoError(t, err)
	assert.NotEmpty(t, new_group_id)

	err = store.AddAnomalyID(ctx, new_group_id,
		"b1fb4036-1883-4d9e-85d4-ed60762901=")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid UUID value")
}

func TestAddCulpitIDs(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()

	new_group_id, err := store.Create(ctx, "sub", "rev-abc", "domain-a", "benchmark-a", 100, 200, "REPORT")
	require.NoError(t, err)
	assert.NotEmpty(t, new_group_id)

	err = store.AddCulpritIDs(ctx, new_group_id,
		[]string{"ffd48105-ce5a-425e-982a-fb4221c46f21"})
	require.NoError(t, err)
	err = store.AddCulpritIDs(ctx, new_group_id,
		[]string{
			"8b4b1f1a-0c26-4c1c-a1c5-e938da8ab072",
			"9e828fc2-063b-40b8-947f-412883b0c82e"})
	require.NoError(t, err)

	group, err2 := store.LoadById(ctx, new_group_id)
	require.NoError(t, err2)
	assert.Equal(t, []string{
		"ffd48105-ce5a-425e-982a-fb4221c46f21",
		"8b4b1f1a-0c26-4c1c-a1c5-e938da8ab072",
		"9e828fc2-063b-40b8-947f-412883b0c82e"}, group.CulpritIds)
}

func TestAddCulpitIDs_InvalidID(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()

	new_group_id, err := store.Create(ctx, "sub", "rev-abc", "domain-a", "benchmark-a", 100, 200, "REPORT")
	require.NoError(t, err)
	assert.NotEmpty(t, new_group_id)

	err = store.AddCulpritIDs(ctx, new_group_id,
		[]string{
			"8b4b1f1a-0c26-4c1c-a1c5-e938da8ab0=",
			"9e828fc2-063b-40b8-947f-412883b0c82e"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid UUID value")
}

// This is the placeholder for the deduplicate work in the future.
// Currently we do not check existing IDs before merging.
func TestAddIDs_DuplicateIDs(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()

	new_group_id, err := store.Create(ctx, "sub", "rev-abc", "domain-a", "benchmark-a", 100, 200, "REPORT")
	require.NoError(t, err)
	assert.NotEmpty(t, new_group_id)

	err = store.AddAnomalyID(ctx, new_group_id,
		"b1fb4036-1883-4d9e-85d4-ed607629017a")
	require.NoError(t, err)
	err = store.AddAnomalyID(ctx, new_group_id,
		"b1fb4036-1883-4d9e-85d4-ed607629017a")
	require.NoError(t, err)
	group, err2 := store.LoadById(ctx, new_group_id)
	require.NoError(t, err2)
	assert.Equal(t, []string{
		"b1fb4036-1883-4d9e-85d4-ed607629017a",
		"b1fb4036-1883-4d9e-85d4-ed607629017a"}, group.AnomalyIds)

	err = store.AddCulpritIDs(ctx, new_group_id,
		[]string{"ffd48105-ce5a-425e-982a-fb4221c46f21"})
	require.NoError(t, err)
	err = store.AddCulpritIDs(ctx, new_group_id,
		[]string{"ffd48105-ce5a-425e-982a-fb4221c46f21"})
	require.NoError(t, err)
	group, err2 = store.LoadById(ctx, new_group_id)
	require.NoError(t, err2)
	assert.Equal(t, []string{
		"ffd48105-ce5a-425e-982a-fb4221c46f21",
		"ffd48105-ce5a-425e-982a-fb4221c46f21"}, group.CulpritIds)
}

func TestFindGroup_RangeDiff(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()

	_, err := store.Create(ctx, "sub", "rev-abc", "domain-a", "benchmark-a", 100, 200, "REPORT")
	require.NoError(t, err)
	_, err = store.Create(ctx, "sub", "rev-abc", "domain-a", "benchmark-a", 200, 300, "REPORT")
	require.NoError(t, err)

	groups, err2 := store.FindExistingGroup(ctx, "sub", "rev-abc", "domain-a", "benchmark-a", 100, 150, "REPORT")
	require.NoError(t, err2)
	assert.Equal(t, 1, len(groups))
}

func TestFindGroup_EmptyString(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()

	_, err := store.Create(ctx, "sub", "rev-abc", "domain-a", "benchmark-a", 100, 200, "REPORT")
	require.NoError(t, err)

	_, err = store.FindExistingGroup(ctx, "", "rev-abc", "domain-a", "benchmark-a", 100, 150, "REPORT")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid params")
}

func TestFindGroup_InvalidCommit(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()

	_, err := store.Create(ctx, "sub", "rev-abc", "domain-a", "benchmark-a", 100, 200, "REPORT")
	require.NoError(t, err)

	_, err = store.FindExistingGroup(ctx, "sub", "rev-abc", "domain-a", "benchmark-a", 0, 150, "REPORT")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid params")
}

func TestGetAnomalyIdsByIssueId(t *testing.T) {
	store, _ := setUp(t)
	ctx := context.Background()

	new_group_id_1, err := store.Create(ctx, "sub", "rev-abc", "domain-a", "benchmark-a", 100, 200, "REPORT")
	require.NoError(t, err)
	new_group_id_2, err := store.Create(ctx, "sub", "rev-abc", "domain-a", "benchmark-a", 100, 200, "REPORT")
	require.NoError(t, err)
	new_group_id_other_issue, err := store.Create(ctx, "sub", "rev-abc", "domain-a", "benchmark-a", 100, 200, "REPORT")
	require.NoError(t, err)

	issueId := "24fa5591-946b-44e4-bf09-3fd271588ee5"
	err = store.UpdateReportedIssueID(ctx, new_group_id_1, issueId)
	require.NoError(t, err)
	err = store.UpdateReportedIssueID(ctx, new_group_id_2, issueId)
	require.NoError(t, err)

	otherIssueId := "9fc415f0-42cf-4606-9a78-d2b9868a1f41"
	err = store.UpdateReportedIssueID(ctx, new_group_id_other_issue, otherIssueId)
	require.NoError(t, err)

	anomaly_id_1 := "b1fb4036-1883-4d9e-85d4-ed607629017a"
	anomaly_id_2 := "a60414c6-2495-4ef7-834a-829b1a929100"
	anomaly_id_3 := "a1235d05-1512-fe41-cba8-32905ec2049a"
	err = store.AddAnomalyID(ctx, new_group_id_1, anomaly_id_1)
	require.NoError(t, err)
	err = store.AddAnomalyID(ctx, new_group_id_2, anomaly_id_2)
	require.NoError(t, err)
	err = store.AddAnomalyID(ctx, new_group_id_2, anomaly_id_3)
	require.NoError(t, err)

	anomaly_id_other_issue := "204cdc89-2ca2-4897-b8e9-82e8058b4330"
	err = store.AddAnomalyID(ctx, new_group_id_other_issue, anomaly_id_other_issue)
	require.NoError(t, err)

	anomaly_ids, err := store.GetAnomalyIdsByIssueId(ctx, issueId)
	require.NoError(t, err)
	// In this test, groups 1 and 2 have the same issue id. Anomalies belonging to them are 1, 2 and 3.
	assert.ElementsMatch(t, []string{anomaly_id_1, anomaly_id_2, anomaly_id_3}, anomaly_ids)
}

func TestGetAnomalyIdsByIssueId_EmptyAnomalyList(t *testing.T) {
	// No anomalies for groups with this issue id.
	store, _ := setUp(t)
	ctx := context.Background()

	new_group_id_1, err := store.Create(ctx, "sub", "rev-abc", "domain-a", "benchmark-a", 100, 200, "REPORT")
	require.NoError(t, err)
	new_group_id_2, err := store.Create(ctx, "sub", "rev-abc", "domain-a", "benchmark-a", 100, 200, "REPORT")
	require.NoError(t, err)
	new_group_id_other_issue, err := store.Create(ctx, "sub", "rev-abc", "domain-a", "benchmark-a", 100, 200, "REPORT")
	require.NoError(t, err)

	issueId := "24fa5591-946b-44e4-bf09-3fd271588ee5"
	err = store.UpdateReportedIssueID(ctx, new_group_id_1, issueId)
	require.NoError(t, err)
	err = store.UpdateReportedIssueID(ctx, new_group_id_2, issueId)
	require.NoError(t, err)

	otherIssueId := "9fc415f0-42cf-4606-9a78-d2b9868a1f41"
	err = store.UpdateReportedIssueID(ctx, new_group_id_other_issue, otherIssueId)
	require.NoError(t, err)

	anomaly_id_other_issue := "204cdc89-2ca2-4897-b8e9-82e8058b4330"
	err = store.AddAnomalyID(ctx, new_group_id_other_issue, anomaly_id_other_issue)
	require.NoError(t, err)

	anomaly_ids, err := store.GetAnomalyIdsByIssueId(ctx, issueId)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{}, anomaly_ids)
}
