package bbstate

import (
	"fmt"
	"sync"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/golden/go/tryjobstore"
	gstorage "google.golang.org/api/storage/v1"
)

func TestBuildBucketState(t *testing.T) {
	testutils.SmallTest(t)

	// TODO(stephana): This test should be tested shomehow, probably by running
	// the simulator in the bot.
	t.Skip()

	// Get the client to be used to access GCS and the Monorail issue tracker.
	serviceAccountFile := "./service-account.json"
	client, err := auth.NewJWTServiceAccountClient("", serviceAccountFile, nil, gstorage.CloudPlatformScope, "https://www.googleapis.com/auth/userinfo.email")
	if err != nil {
		sklog.Fatalf("Failed to authenticate service account: %s", err)
	}

	tjStore, err := tryjobstore.NewCloudTryjobStore(common.PROJECT_ID, "gold_ingestion-localhost-stephana", serviceAccountFile)
	assert.NoError(t, err)

	// Remove all issues.
	deleteAllIssues(t, tjStore)
	if tjStore != nil {
		return
	}
	defer func() {
		deleteAllIssues(t, tjStore)
	}()

	gerritAPI, err := gerrit.NewGerrit(gerrit.GERRIT_SKIA_URL, "", client)
	assert.NoError(t, err)

	bbConf := &Config{
		BuildBucketURL:  DefaultSkiaBuildBucketURL,
		BuildBucketName: DefaultSkiaBucketName,
		Client:          client,
		TryjobStore:     tjStore,
		GerritClient:    gerritAPI,
		PollInterval:    time.Hour,
		TimeWindow:      5 * time.Hour,
	}

	tjStatus, err := NewBuildBucketState(bbConf)
	assert.NoError(t, err)
	assert.NotNil(t, tjStatus)

	time.Sleep(10 * time.Second)

	// Output all the issue we have found.
	issues, _, err := tjStore.ListIssues()
	assert.NoError(t, err)
	for _, issueEntry := range issues {
		if issueEntry.ID == 54204 {
			issue, err := tjStore.GetIssue(issueEntry.ID, true, nil)
			assert.NoError(t, err)

			fmt.Printf("%s - %s\n", issue.Subject, issue.Owner)
			for _, patchset := range issue.PatchsetDetails {
				fmt.Printf("Patchset %d: ", patchset.ID)
				fmt.Printf(" %d tryjobs.\n", len(patchset.Tryjobs))
				for _, tj := range patchset.Tryjobs {
					fmt.Printf("     %s\n", tj.String())
				}
			}
		}
	}
}

func deleteAllIssues(t *testing.T, tjStore tryjobstore.TryjobStore) {
	issues, _, err := tjStore.ListIssues()
	assert.NoError(t, err)
	fmt.Printf("Got %d issues\n", len(issues))

	perm := make(chan bool, 10)
	var wg sync.WaitGroup
	for _, entry := range issues {
		wg.Add(1)
		go func(entry *tryjobstore.Issue) {
			perm <- true
			defer wg.Done()
			if err := tjStore.DeleteIssue(entry.ID); err != nil {
				fmt.Printf("ERROR: %s\n", err)
			}
			<-perm
		}(entry)
	}
	wg.Wait()
}
