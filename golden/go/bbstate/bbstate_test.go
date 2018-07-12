package bbstate

import (
	"fmt"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/ds"
	ds_testutil "go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/golden/go/tryjobstore"
	gstorage "google.golang.org/api/storage/v1"
)

func TestBuildBucketState(t *testing.T) {
	testutils.SmallTest(t)

	// Uncomment the line below to run local tests.
	// t.Skip()

	// Get the client to be used to access GCS and the Monorail issue tracker.
	httpClient, err := auth.NewJWTServiceAccountClient("", "", nil, gstorage.CloudPlatformScope, "https://www.googleapis.com/auth/userinfo.email")
	if err != nil {
		sklog.Fatalf("Failed to authenticate service account: %s", err)
	}

	// Otherwise try and connect to a locally running emulator.
	cleanup := ds_testutil.InitDatastore(t,
		ds.ISSUE,
		ds.TRYJOB,
		ds.TRYJOB_RESULT,
		ds.TRYJOB_EXP_CHANGE,
		ds.TEST_DIGEST_EXP)
	defer cleanup()
	dsClient := ds.DS

	evt := eventbus.New()
	tjStore, err := tryjobstore.NewCloudTryjobStore(dsClient, evt)
	assert.NoError(t, err)

	_, err = ds.DeleteAll(dsClient, ds.ISSUE, true)
	assert.NoError(t, err)

	gerritAPI, err := gerrit.NewGerrit(gerrit.GERRIT_SKIA_URL, "", httpClient)
	assert.NoError(t, err)

	bbConf := &Config{
		BuildBucketURL:  DefaultSkiaBuildBucketURL,
		BuildBucketName: DefaultSkiaBucketName,
		Client:          httpClient,
		TryjobStore:     tjStore,
		GerritClient:    gerritAPI,
		PollInterval:    10 * time.Second,
		TimeWindow:      3 * time.Hour,
	}

	tjStatus, err := NewBuildBucketState(bbConf)
	assert.NoError(t, err)
	assert.NotNil(t, tjStatus)
	bbState := tjStatus.(*BuildBucketState)

	// Wait for at least two search cycles to fetch some builds.
	time.Sleep(buildWatcherPollInterval)

	initialWatching := bbState.GetWatchedBuilds()
	for {
		current := bbState.GetWatchedBuilds()
		fmt.Printf("Currently watching %d builds \n", len(bbState.GetWatchedBuilds()))
		stillRunning := 0
		for id := range current {
			if _, ok := initialWatching[id]; ok {
				stillRunning++
			}
		}
		if stillRunning > 0 {
			fmt.Printf("Of initial builds %d are still running. Continue to wait.\n", stillRunning)
			time.Sleep(10 * time.Second)
		} else {
			break
		}
	}

	// Output all the issue we have found.
	issues, _, err := tjStore.ListIssues(0, 1000)
	assert.NoError(t, err)
	for _, issueEntry := range issues {
		issue, err := tjStore.GetIssue(issueEntry.ID, true)
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

// func deleteAllIssues(t *testing.T, tjStore tryjobstore.TryjobStore) {
// 	issues, _, err := tjStore.ListIssues(0, 1000)
// 	assert.NoError(t, err)
// 	fmt.Printf("Got %d issues\n", len(issues))

// 	perm := make(chan bool, 10)
// 	var wg sync.WaitGroup
// 	for _, entry := range issues {
// 		wg.Add(1)
// 		go func(entry *tryjobstore.Issue) {
// 			perm <- true
// 			defer wg.Done()
// 			if err := tjStore.DeleteIssue(entry.ID); err != nil {
// 				fmt.Printf("ERROR: %s\n", err)
// 			}
// 			<-perm
// 		}(entry)
// 	}
// 	wg.Wait()
// }
