package roller_cleanup

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const rollerID = "my-roller"

var arbitraryTime = time.Unix(1715005596, 0)

func makeRequest() *CleanupRequest {
	return &CleanupRequest{
		RollerID:      rollerID,
		NeedsCleanup:  true,
		User:          "me@google.com",
		Timestamp:     arbitraryTime,
		Justification: "needs cleanup",
	}
}

func TestCleanupRequestValidation(t *testing.T) {
	req := makeRequest()
	require.NoError(t, req.Validate())

	req = makeRequest()
	req.RollerID = ""
	require.ErrorContains(t, req.Validate(), "RollerID is required")

	req = makeRequest()
	req.NeedsCleanup = false
	require.NoError(t, req.Validate()) // Failing in this case makes no sense.

	req = makeRequest()
	req.User = ""
	require.ErrorContains(t, req.Validate(), "User is required")

	req = makeRequest()
	req.Timestamp = time.Time{}
	require.ErrorContains(t, req.Validate(), "Timestamp is required")

	req = makeRequest()
	req.Timestamp = time.Unix(0, 0)
	require.ErrorContains(t, req.Validate(), "Timestamp is required")

	req = makeRequest()
	req.Justification = ""
	require.ErrorContains(t, req.Validate(), "Justification is required")
}

func testNeedsCleanup(t *testing.T, db DB) {
	ctx := context.Background()

	// In the initially-empty state, we have no cleanup requests and therefore
	// we do not need cleanup.
	needsCleanup, err := NeedsCleanup(ctx, db, rollerID)
	require.NoError(t, err)
	require.False(t, needsCleanup)

	// Add a request for cleanup.
	req := makeRequest()
	req.NeedsCleanup = true
	require.NoError(t, db.RequestCleanup(ctx, req))

	// Now we need cleanup.
	needsCleanup, err = NeedsCleanup(ctx, db, rollerID)
	require.NoError(t, err)
	require.True(t, needsCleanup)

	// Add a request for cleanup.
	req = makeRequest()
	req.NeedsCleanup = false
	require.NoError(t, db.RequestCleanup(ctx, req))

	// We no longer need cleanup.
	needsCleanup, err = NeedsCleanup(ctx, db, rollerID)
	require.NoError(t, err)
	require.False(t, needsCleanup)
}

func testDB_History(t *testing.T, db DB) {
	ctx := context.Background()

	// Cleanup requests, in forward chronological order.
	reqs := []*CleanupRequest{
		{
			RollerID:      rollerID,
			NeedsCleanup:  true,
			User:          "me@google.com",
			Timestamp:     time.Unix(1715000000, 0).UTC(),
			Justification: "want cleanup",
		},
		{
			RollerID:      rollerID,
			NeedsCleanup:  false,
			User:          "me@google.com",
			Timestamp:     time.Unix(1715000100, 0).UTC(),
			Justification: "no longer want cleanup",
		},
		{
			RollerID:      rollerID,
			NeedsCleanup:  true,
			User:          "me@google.com",
			Timestamp:     time.Unix(1715000200, 0).UTC(),
			Justification: "want cleanup",
		},
		{
			RollerID:      rollerID,
			NeedsCleanup:  false,
			User:          "me@google.com",
			Timestamp:     time.Unix(1715000300, 0).UTC(),
			Justification: "no longer want cleanup",
		},
		{
			RollerID:      rollerID,
			NeedsCleanup:  true,
			User:          "me@google.com",
			Timestamp:     time.Unix(1715000400, 0).UTC(),
			Justification: "want cleanup",
		},
		{
			RollerID:      rollerID,
			NeedsCleanup:  false,
			User:          "me@google.com",
			Timestamp:     time.Unix(1715000500, 0).UTC(),
			Justification: "no longer want cleanup",
		},
		{
			RollerID:      rollerID,
			NeedsCleanup:  true,
			User:          "me@google.com",
			Timestamp:     time.Unix(1715000600, 0).UTC(),
			Justification: "want cleanup",
		},
		{
			RollerID:      rollerID,
			NeedsCleanup:  false,
			User:          "me@google.com",
			Timestamp:     time.Unix(1715000700, 0).UTC(),
			Justification: "no longer want cleanup",
		},
	}

	// Add the requests to the DB.
	for _, req := range reqs {
		require.NoError(t, db.RequestCleanup(ctx, req))
	}

	// Create a reversed slice of requests, which will be handy for testing.
	reversedReqs := make([]*CleanupRequest, 0, len(reqs))
	for i := len(reqs) - 1; i >= 0; i-- {
		reversedReqs = append(reversedReqs, reqs[i])
	}

	// Zero or negative limit gives us all requests.
	history, err := db.History(ctx, rollerID, 0)
	require.NoError(t, err)
	require.Equal(t, reversedReqs, history)

	history, err = db.History(ctx, rollerID, -100)
	require.NoError(t, err)
	require.Equal(t, reversedReqs, history)

	// Otherwise, we respect the given limit.
	history, err = db.History(ctx, rollerID, 1)
	require.NoError(t, err)
	require.Equal(t, reversedReqs[0:1], history)

	history, err = db.History(ctx, rollerID, 5)
	require.NoError(t, err)
	require.Equal(t, reversedReqs[0:5], history)
}
