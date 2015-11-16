// query_issues is a simple command-line application for querying the monorail issue tracker.
//
// It takes a single command-line argument which is the text to search for.
package main

import (
	"fmt"
	"os"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/issues"
)

func main() {
	client, err := auth.NewDefaultClient(true, "https://www.googleapis.com/auth/userinfo.email")
	if err != nil {
		glog.Fatalf("Unable to create installed app oauth client:%s", err)
	}
	tracker := issues.NewMonorailIssueTracker(client)
	if len(os.Args) != 2 {
		glog.Fatalf("query_issues takes a single argument which is the query to perform.")
	}
	iss, err := tracker.FromQuery(os.Args[1])
	if err != nil {
		fmt.Printf("Failed to retrieve issues: %s", err)
		return
	}
	fmt.Printf("Found: %d", len(iss))
	for _, issue := range iss {
		fmt.Printf("%20d %10s %s\n", issue.ID, issue.State, issue.Title)
	}
}
