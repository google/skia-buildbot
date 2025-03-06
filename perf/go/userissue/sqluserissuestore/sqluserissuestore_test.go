package sqluserissuestore

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/perf/go/sql/sqltest"
	"go.skia.org/infra/perf/go/userissue"
)

func setUp(t *testing.T) (userissue.Store, pool.Pool) {
	db := sqltest.NewSpannerDBForTests(t, "userissuestore")
	store := New(db)
	return store, db
}

func TestSave_UserIssue_Success(t *testing.T) {
	ctx := context.Background()
	store, _ := setUp(t)
	ui := userissue.UserIssue{
		UserId:         "a@b.com",
		TraceKey:       ",a=1,b=1,c=1,",
		CommitPosition: 1,
		IssueId:        12345,
	}
	err := store.Save(ctx, &ui)
	require.NoError(t, err)
	userissues, err := store.GetUserIssuesForTraceKeys(ctx, []string{",a=1,b=1,c=1,"}, 1, 1)
	require.NoError(t, err)
	require.Len(t, userissues, 1)
	require.EqualValues(t, ui, userissues[0])
}

func TestSaveUserIssue_Duplicate_Failure(t *testing.T) {
	ctx := context.Background()
	store, _ := setUp(t)
	ui := userissue.UserIssue{
		UserId:         "a@b.com",
		TraceKey:       ",a=1,b=1,c=1,",
		CommitPosition: 1,
		IssueId:        12345,
	}
	err := store.Save(ctx, &ui)
	require.NoError(t, err)

	ui1 := userissue.UserIssue{
		UserId:         "a@b.com",
		TraceKey:       ",a=1,b=1,c=1,",
		CommitPosition: 1,
		IssueId:        12345,
	}
	err = store.Save(ctx, &ui)
	require.Error(t, err)
	expectedError := fmt.Sprintf("Failed to insert userissue for traceKey=%s and commitPosition=%d", ui1.TraceKey, ui1.CommitPosition)
	require.ErrorContains(t, err, expectedError)
}

func TestDeleteUserIssue_Existing_Success(t *testing.T) {
	ctx := context.Background()
	store, _ := setUp(t)
	ui := userissue.UserIssue{
		UserId:         "a@b.com",
		TraceKey:       ",a=1,b=1,c=1,",
		CommitPosition: 1,
		IssueId:        12345,
	}
	err := store.Save(ctx, &ui)
	require.NoError(t, err)
	userissues, err := store.GetUserIssuesForTraceKeys(ctx, []string{",a=1,b=1,c=1,"}, 1, 1)
	require.NoError(t, err)
	require.Len(t, userissues, 1)

	err = store.Delete(ctx, ui.TraceKey, ui.CommitPosition)
	require.NoError(t, err)

	userissues, err = store.GetUserIssuesForTraceKeys(ctx, []string{",a=1,b=1,c=1,"}, 1, 1)
	require.NoError(t, err)
	require.Len(t, userissues, 0)
}
func TestDeleteUserIssue_NonExisting_Failure(t *testing.T) {
	ctx := context.Background()
	store, _ := setUp(t)
	ui := userissue.UserIssue{
		UserId:         "a@b.com",
		TraceKey:       ",a=1,b=1,c=1,",
		CommitPosition: 1,
		IssueId:        12345,
	}

	err := store.Delete(ctx, ui.TraceKey, ui.CommitPosition)
	require.Error(t, err)
	expectedErr := fmt.Sprintf("No such record exists for trace key=%s and commit position=%d", ui.TraceKey, ui.CommitPosition)
	require.ErrorContains(t, err, expectedErr)
}

func TestGet_UserIssues_Success(t *testing.T) {
	ctx := context.Background()
	store, _ := setUp(t)

	ui1 := &userissue.UserIssue{
		UserId:         "a@b.com",
		TraceKey:       ",a=1,b=1,c=1,",
		CommitPosition: 1,
		IssueId:        1234,
	}
	ui12 := &userissue.UserIssue{
		UserId:         "a@b.com",
		TraceKey:       ",a=1,b=1,c=1,",
		CommitPosition: 2,
		IssueId:        1235,
	}
	ui2 := &userissue.UserIssue{
		UserId:         "b@c.com",
		TraceKey:       ",a=2,b=2,c=2,",
		CommitPosition: 2,
		IssueId:        2345,
	}
	ui22 := &userissue.UserIssue{
		UserId:         "b@c.com",
		TraceKey:       ",a=2,b=2,c=2,",
		CommitPosition: 4,
		IssueId:        2346,
	}
	ui3 := &userissue.UserIssue{
		UserId:         "c@d.com",
		TraceKey:       ",a=3,b=3,c=3,",
		CommitPosition: 3,
		IssueId:        3456,
	}
	ui32 := &userissue.UserIssue{
		UserId:         "c@d.com",
		TraceKey:       ",a=3,b=3,c=3,",
		CommitPosition: 10,
		IssueId:        3457,
	}
	ui33 := &userissue.UserIssue{
		UserId:         "c@d.com",
		TraceKey:       ",a=3,b=3,c=3,",
		CommitPosition: 12,
		IssueId:        4567,
	}
	userIssues := []*userissue.UserIssue{ui1, ui12, ui2, ui22, ui3, ui32, ui33}
	for _, req := range userIssues {
		err := store.Save(ctx, req)
		require.NoError(t, err)
	}
	traceKeys := []string{
		",a=1,b=1,c=1,",
		",a=2,b=2,c=2,",
		",a=3,b=3,c=3,",
	}
	beginCommitPosition := 2
	endCommitPosition := 13
	resp, err := store.GetUserIssuesForTraceKeys(ctx, traceKeys, int64(beginCommitPosition), int64(endCommitPosition))
	require.NoError(t, err)
	expectedUserIssues := []userissue.UserIssue{*ui12, *ui2, *ui22, *ui3, *ui32, *ui33}
	sort.Slice(resp, func(i, j int) bool {
		if resp[i].UserId == resp[j].UserId {
			return resp[i].IssueId < resp[j].IssueId
		}
		return resp[i].UserId < resp[j].UserId
	})
	require.EqualValues(t, expectedUserIssues, resp)
}
