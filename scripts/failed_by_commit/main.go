// Print out the name of every bot that failed in the Skia pool at the given commit.
package main

import (
	"flag"
	"fmt"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
)

var (
	source_revision = flag.String("source_revision", "", "git hash we are looking for failures at.")
)

func main() {
	defer common.LogPanic()
	common.Init()

	if *source_revision == "" {
		sklog.Fatalf("The --source_revision flag is required.")
	}
	httpClient, err := auth.NewDefaultClient(true, swarming.AUTH_SCOPE)
	if err != nil {
		sklog.Fatal(err)
	}
	swarmApi, err := swarming.NewApiClient(httpClient, swarming.SWARMING_SERVER)
	if err != nil {
		sklog.Fatal(err)
	}
	resp, err := swarmApi.ListTasks(time.Time{}, time.Time{}, []string{fmt.Sprintf("source_revision:%s", *source_revision)}, "completed_failure")
	if err != nil {
		sklog.Fatal(err)
	}
	for _, r := range resp {
		fmt.Printf("%s\n", r.TaskResult.Name)
	}

}
