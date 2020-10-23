package bugs

import (
	"testing"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/bugs-central/go/types"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestPutOpenIssues(t *testing.T) {
	unittest.SmallTest(t)

	o := InitOpenIssues()
	client1 := types.RecognizedClient("client1")
	client2 := types.RecognizedClient("client2")
	source1 := types.IssueSource("source1")
	source2 := types.IssueSource("source2")
	query1 := "query1"
	query2 := "query2"
	issues1 := []*types.Issue{
		{Id: "id11"},
		{Id: "id12"},
	}
	issues2 := []*types.Issue{
		{Id: "id21"},
		{Id: "id22"},
		{Id: "id32"},
	}
	issues3 := []*types.Issue{
		{Id: "id31"},
		{Id: "id32"},
		{Id: "id32"},
	}

	// Add 1 client+source+query entry with 2 issues.
	o.PutOpenIssues(client1, source1, query1, issues1)
	require.Equal(t, 1, len(o.openIssues))
	require.Equal(t, 1, len(o.openIssues[client1]))
	require.Equal(t, 1, len(o.openIssues[client1][source1]))
	require.Equal(t, 2, len(o.openIssues[client1][source1][query1]))
	require.Equal(t, "id11", o.openIssues[client1][source1][query1][0].Id)
	require.Equal(t, "id12", o.openIssues[client1][source1][query1][1].Id)

	// Add another client with 2 sources.
	o.PutOpenIssues(client2, source1, query2, issues2)
	o.PutOpenIssues(client2, source2, query2, issues3)
	require.Equal(t, 2, len(o.openIssues))
	require.Equal(t, 2, len(o.openIssues[client2]))
	require.Equal(t, "id21", o.openIssues[client2][source1][query2][0].Id)
	require.Equal(t, "id31", o.openIssues[client2][source2][query2][0].Id)

	// Replace an existing entries with new set of issues.
	o.PutOpenIssues(client2, source1, query2, issues1)
	require.Equal(t, 2, len(o.openIssues[client2][source1][query2]))
	require.Equal(t, "id11", o.openIssues[client2][source1][query2][0].Id)
	require.Equal(t, "id12", o.openIssues[client2][source1][query2][1].Id)
}
