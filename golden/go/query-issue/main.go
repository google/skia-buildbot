package main

import (
	"flag"
	"fmt"
	"strconv"

	"github.com/davecgh/go-spew/spew"

	"google.golang.org/api/option"
	gstorage "google.golang.org/api/storage/v1"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/tryjobstore"
)

// Command line flags.
var (
	dsNamespace        = flag.String("ds_namespace", "", "Cloud datastore namespace to be used by this instance.")
	serviceAccountFile = flag.String("service_account_file", "", "Credentials file for service account.")
)

const (
	IMAGE_URL_PREFIX = "/img/"

	// OAUTH2_CALLBACK_PATH is callback endpoint used for the Oauth2 flow.
	OAUTH2_CALLBACK_PATH = "/oauth2callback/"
)

var (
	// disableIssueQueries controls whether this instance can query tryjob results.
	disableIssueQueries = false
)

func main() {
	common.Init()

	issueStr := flag.Arg(0)
	issueID, err := strconv.Atoi(issueStr)
	if err != nil {
		sklog.Fatalf("Unable to parse issue '%s': %s", issueStr, err)
	}

	// Get the token source from the same service account. Needed to access cloud pubsub and datastore.
	tokenSource, err := auth.NewJWTServiceAccountTokenSource("", *serviceAccountFile, gstorage.CloudPlatformScope)
	if err != nil {
		sklog.Fatalf("Failed to authenticate service account to get token source: %s", err)
	}

	if err := ds.InitWithOpt(common.PROJECT_ID, *dsNamespace, option.WithTokenSource(tokenSource)); err != nil {
		sklog.Fatalf("Unable to configure cloud datastore: %s", err)
	}

	tryjobStore, err := tryjobstore.NewCloudTryjobStore(ds.DS, nil)
	if err != nil {
		sklog.Fatalf("Unable to instantiate tryjob store: %s", err)
	}

	issue, err := tryjobStore.GetIssue(int64(issueID), true, nil)
	if err != nil {
		sklog.Fatalf("Error fetching issue %d: %s", issueID, err)
	}

	fmt.Printf("Issue:%s\n", spew.Sdump(issue))
}
