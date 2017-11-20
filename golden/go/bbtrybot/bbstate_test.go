package bbtrybot

import (
	"fmt"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/sklog"
	gstorage "google.golang.org/api/storage/v1"
)

func TestTrybotStatus(t *testing.T) {
	// Get the client to be used to access GCS and the Monorail issue tracker.
	serviceAccountFile := "./service-account.json"
	client, err := auth.NewJWTServiceAccountClient("", serviceAccountFile, nil, gstorage.CloudPlatformScope, "https://www.googleapis.com/auth/userinfo.email")
	if err != nil {
		sklog.Fatalf("Failed to authenticate service account: %s", err)
	}

	// oauthCacheFile := "./google_storage_token.data"
	// httpClient, err := auth.NewClient(true, oauthCacheFile, auth.SCOPE_READ_WRITE)
	// if err != nil {
	// 	sklog.Fatal(err)
	// }

	memTrybotStore := NewMemTrybotStore()
	gerritAPI, err := gerrit.NewGerrit(gerrit.GERRIT_SKIA_URL, "", client)
	assert.NoError(t, err)

	tbStatus, err := NewTrybotState(client, memTrybotStore, gerritAPI)
	assert.NoError(t, err)
	assert.NotNil(t, tbStatus)

	time.Sleep(10 * time.Second)

	// Output all the issue we have found.
	issues, _, err := memTrybotStore.ListTrybotIssues(0, 0)
	assert.NoError(t, err)
	for _, issueEntry := range issues {
		issue, err := memTrybotStore.GetIssue(issueEntry.ID, nil)
		assert.NoError(t, err)
		fmt.Printf("%s - %s\n", issue.Subject, issue.Owner)
		for idx, patchsetID := range issue.Patchsets {
			fmt.Printf("Patchset %d: ", idx)
			if ps, ok := issue.PatchsetDetails[patchsetID]; !ok {
				fmt.Printf(" MISSING\n")
			} else {
				fmt.Printf(" %d tryjobs.\n", len(ps.Tryjobs))
			}
		}
	}
}
