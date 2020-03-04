package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
)

var (
	// Flags.
	// ADD TIME
	// WHat about user???????? can you find who the user is by yourself??? what about me............

	gerritInstance = flag.String("gerrit_instance", "skia-review", "Name of the gerrit instance. Eg: skia-review.")

	//deleteBranch   = flag.String("delete", "", "Name of an existing branch to delete, without refs/heads prefix.")
	//excludeTrybots = common.NewMultiStringFlag("exclude-trybots", nil, "Regular expressions for trybot names to exclude.")
	//owner          = flag.String("owner", "", "Owner of the new branch.")
	//repoUrl        = flag.String("repo", common.REPO_SKIA, "URL of the git repository.")
	//submit         = flag.Bool("submit", false, "If set, automatically submit the CL to update the CQ and supported branches.")
)

func printIssue(i *gerrit.ChangeInfo, gUrl string, num int) {
	fmt.Printf("-----------------#%d------------------\n", num)
	fmt.Printf("Gerrit CL   : %s/c/%s/+/%d\n", gUrl, i.Project, i.Issue)
	fmt.Printf("Subject     : %s\n", i.Subject)
	//fmt.Printf("Repo Branch : %s %s\n", i.Project, i.Branch)
	fmt.Printf("Updated at  : %s\n", i.Updated)
	fmt.Println("-------------------------------------")
}

func printIssues(issues []*gerrit.ChangeInfo, gUrl string) {
	fmt.Println("\n")
	for idx, i := range issues {
		printIssue(i, gUrl, idx+1)
	}
	fmt.Println("\n")
}

func main() {
	common.Init()
	ctx := context.Background()

	if *gerritInstance == "" {
		sklog.Fatal("--gerrit_instance is required.")
	}
	//newRef := fmt.Sprintf("refs/heads/%s", *branch)
	//if *owner == "" {
	//	sklog.Fatal("--owner is required.")
	//}
	//excludeTrybotRegexp := make([]*regexp.Regexp, 0, len(*excludeTrybots))
	//for _, excludeTrybot := range *excludeTrybots {
	//	re, err := regexp.Compile(excludeTrybot)
	//	if err != nil {
	//		sklog.Fatalf("Failed to compile regular expression from %q; %s", excludeTrybot, err)
	//	}
	//	excludeTrybotRegexp = append(excludeTrybotRegexp, re)
	//}

	//// Setup.
	//wd, err := ioutil.TempDir("", "new-branch")
	//if err != nil {
	//	sklog.Fatal(err)
	//}
	//defer util.RemoveAll(wd)

	ts, err := auth.NewDefaultTokenSource(true, auth.SCOPE_GERRIT)
	if err != nil {
		sklog.Fatal(err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).Client()
	gUrl := fmt.Sprintf("https://%s.googlesource.com", *gerritInstance)
	g, err := gerrit.NewGerrit(gUrl, client)
	if err != nil {
		sklog.Fatal(err)
	}

	issues, err := g.Search(ctx, 100, true, gerrit.SearchOwner("rmistry@google.com"), gerrit.SearchStatus(gerrit.CHANGE_STATUS_NEW))
	// , gerrit.SearchStatus(gerrit.CHANGE_STATUS_DRAFT)
	// gerrit.SearchStatus(gerrit.CHANGE_STATUS_NEW)
	if err != nil {
		sklog.Fatalf("Failed to retrieve issues: %s", err)
	}

	if len(issues) == 0 {
		fmt.Println("Found 0 issues.")
		return
	} else {
		printIssues(issues, gUrl)
	}

	fmt.Printf("Found %d issues.\n\n", len(issues))
	fmt.Println("[1] Abandon all.")
	fmt.Println("[2] Abandon selectively. You will be prompted for each issue.")
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("\nEnter one option [1,2] ")
	char, _, err := reader.ReadRune()
	if err != nil {
		sklog.Fatalf("Could not read input: %s", err)
	}
	switch char {
	case '1':
		fmt.Println("ABANDON ALL OF THEM VIA THE API!")
		break
	case '2':
		for idx, i := range issues {
			printIssue(i, gUrl, idx+1)
			reader = bufio.NewReader(os.Stdin)
			fmt.Println("Abandon? [y,n]")
			char, _, err := reader.ReadRune()
			if err != nil {
				sklog.Fatalf("Could not read input: %s", err)
			}
			switch char {
			case 'y':
				fmt.Println("Going to abandon this guy!")
				break
			case 'n':
				fmt.Println("Skipping this guy!")
				break
			default:
				fmt.Println("Could not recognize input. Exiting.")
				return
			}
		}
	default:
		fmt.Println("Could not recognize input. Exiting.")
		return
	}
}
