// issues_helper is a simple command-line application for the monorail issue tracker.
//
// It can be used in a few modes, for example:
// issues_helper comment 5255 "Spectacular flooring"
// issues_helper query "is:open label:FromSkiaPerf"
package main

import (
	"flag"
	"fmt"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/issues"
	"go.skia.org/infra/go/sklog"
)

var (
	tracker issues.IssueTracker

	status      = flag.String("status", "New", "The status of the newly created issue")
	summary     = flag.String("summary", "", "The summary/title of the newly created issue")
	description = flag.String("description", "", "The description of the newly created issue")
	owner       = flag.String("owner", "", "The email address of the issue's owner")
	cc          = common.NewMultiStringFlag("cc", nil, fmt.Sprintf("The email addresses to cc to this issue."))
	labels      = common.NewMultiStringFlag("labels", nil, fmt.Sprintf(`The labels to add to this issue.  Common labels are "Type-Defect","Priority-Medium","Restrict-View-Google"`))
	component   = flag.String("component", "", "Component name under which to file the issue.")
	project     = flag.String("project", "", "Project ID under which to file the issue.")
)

func main() {
	common.Init()
	args := flag.Args()
	if len(args) < 1 {
		fmt.Println("Usage: issues_helper <mode> [OPTIONS]")
		return
	}
	mode := args[0]

	ts, err := auth.NewDefaultTokenSource(true, "https://www.googleapis.com/auth/userinfo.email")
	if err != nil {
		sklog.Fatalf("Unable to create token source: %s", err)
	}
	if *project == "" {
		*project = issues.PROJECT_SKIA
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	tracker = issues.NewMonorailIssueTracker(client, *project)
	switch mode {
	case "query":
		if len(args) < 2 {
			fmt.Println("Usage: issues_helper query <query_string>")
			return
		}
		handleQuery(args[1:])
	case "comment":
		if len(args) < 4 {
			fmt.Println("Usage: issues_helper comment <issue_id> <comment_string>")
			return
		}
		handleComment(args[1:])
	case "create":
		if len(args) != 1 || !checkCreateFlags() {
			fmt.Println("Usage: issues_helper [--options=foo,bar] create")
			fmt.Println("Required flags: --status, --summary, --description")
			fmt.Println("Optional flags: --owner, --labels, --cc")
			return
		}
		handleCreate()
	}
}

func handleQuery(args []string) {
	query := args[0]
	iss, err := tracker.FromQuery(query)
	if err != nil {
		sklog.Errorf("Failed to retrieve issues: %s\n", err)
		return
	}
	fmt.Printf("Found: %d\n", len(iss))
	for _, issue := range iss {
		sklog.Infof("%20d %10s %s\n", issue.ID, issue.State, issue.Title)
	}
}

func handleComment(args []string) {
	id := args[0]
	comment := issues.CommentRequest{
		Content: args[1],
	}
	if err := tracker.AddComment(id, comment); err != nil {
		sklog.Errorf("Failed to add comment: %s\n", err)
		return
	}
}

func handleCreate() {
	ccPeople := make([]issues.MonorailPerson, 0)
	for _, p := range *cc {
		ccPeople = append(ccPeople, issues.MonorailPerson{
			Name: p,
		})
	}
	req := issues.IssueRequest{
		Status: *status,
		Owner: issues.MonorailPerson{
			Name: *owner,
		},
		CC:          ccPeople,
		Labels:      *labels,
		Summary:     *summary,
		Description: *description,
		Components:  []string{*component},
	}
	if err := tracker.AddIssue(req); err != nil {
		sklog.Errorf("Failed to add issue: %s", err)
	}
}

func checkCreateFlags() bool {
	return *status != "" && *summary != "" && *description != ""
}
