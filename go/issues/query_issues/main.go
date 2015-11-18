// query_issues is a simple command-line application for querying the monorail issue tracker.
//
// It takes a single command-line argument which is the text to search for.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/issues"
)

var (
	local = flag.Bool("local", true, "Running locally if true. As opposed to in production.")
)

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("Usage: query_issues <query> [OPTIONS]")
		return
	}
	query := os.Args[1]
	os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
	common.Init()
	client, err := auth.NewDefaultClient(*local, "https://www.googleapis.com/auth/userinfo.email")
	if err != nil {
		glog.Fatalf("Unable to create installed app oauth client:%s", err)
	}
	tracker := issues.NewMonorailIssueTracker(client)
	iss, err := tracker.FromQuery(query)
	if err != nil {
		fmt.Printf("Failed to retrieve issues: %s", err)
		return
	}
	fmt.Printf("Found: %d", len(iss))
	for _, issue := range iss {
		fmt.Printf("%20d %10s %s\n", issue.ID, issue.State, issue.Title)
	}
}
