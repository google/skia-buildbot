package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
)

var (
	gerritInstance         = flag.String("gerrit_instance", "https://skia-review.googlesource.com", "Name of the gerrit instance")
	abandonReason          = flag.String("abandon_reason", "", "Optional. Will be used as reason for abandoning")
	lastModifiedBeforeDays = flag.Int("last_modified_before_days", 0, "Optional. If '3' is specified then all CLs that were modified after 3 days ago will be returned")
)

func printIssue(i *gerrit.ChangeInfo, gUrl string, num int) {
	fmt.Printf("\n#%d\n", num)
	fmt.Printf("\tGerrit CL : %s/c/%s/+/%d\n", gUrl, i.Project, i.Issue)
	fmt.Printf("\tSubject   : %s\n", i.Subject)
	fmt.Printf("\tUpdated   : %s\n\n", i.Updated)
}

func printIssues(issues []*gerrit.ChangeInfo, gUrl string) {
	fmt.Println()
	for idx, i := range issues {
		printIssue(i, gUrl, idx+1)
	}
	fmt.Println()
}

func abandonIssue(ctx context.Context, i *gerrit.ChangeInfo, g *gerrit.Gerrit) error {
	return g.Abandon(ctx, i, *abandonReason)
}

func main() {
	common.Init()
	ctx := context.Background()

	if *gerritInstance == "" {
		sklog.Fatal("--gerrit_instance is required.")
	}

	ts, err := auth.NewDefaultTokenSource(true, auth.ScopeGerrit)
	if err != nil {
		sklog.Fatal(err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).Client()
	g, err := gerrit.NewGerrit(*gerritInstance, client)
	if err != nil {
		sklog.Fatal(err)
	}

	searchTerms := []*gerrit.SearchTerm{gerrit.SearchOwner("me"), gerrit.SearchStatus(gerrit.ChangeStatusNew)}
	if *lastModifiedBeforeDays != 0 {
		beforeHours := *lastModifiedBeforeDays * 24
		searchTerms = append(searchTerms, gerrit.SearchModifiedAfter(time.Now().Add(-time.Duration(beforeHours)*time.Hour)))
	}
	issues, err := g.Search(ctx, 100, false, searchTerms...)
	// Reverse the slice to keep the oldest issues first
	for i, j := 0, len(issues)-1; i < j; i, j = i+1, j-1 {
		issues[i], issues[j] = issues[j], issues[i]
	}

	if err != nil {
		sklog.Fatalf("Failed to retrieve issues: %s", err)
	}

	if len(issues) == 0 {
		fmt.Println("Found 0 issues.")
		return
	} else {
		fmt.Printf("\nFound %d issues (displaying oldest first):", len(issues))
		printIssues(issues, *gerritInstance)
	}

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
		reader = bufio.NewReader(os.Stdin)
		fmt.Print("Are you sure? [y,n]: ")
		char, _, err := reader.ReadRune()
		if err != nil {
			sklog.Fatalf("Could not read input: %s", err)
		}
		switch char {
		case 'y':
			for _, i := range issues {
				if err := abandonIssue(ctx, i, g); err != nil {
					sklog.Fatalf("Could not abandon %d: %s", i.Issue, err)
				}
				fmt.Printf("Abandoned %d\n", i.Issue)
			}
			break
		case 'n':
			fmt.Println("Not abandoning any issues. Exiting.")
			return
		default:
			fmt.Println("Could not recognize input. Exiting.")
			return
		}
		break
	case '2':
		for idx, i := range issues {
			printIssue(i, *gerritInstance, idx+1)
			reader = bufio.NewReader(os.Stdin)
			fmt.Print("Abandon? [y,n]: ")
			char, _, err := reader.ReadRune()
			if err != nil {
				sklog.Fatalf("Could not read input: %s", err)
			}
			switch char {
			case 'y':
				if err := abandonIssue(ctx, i, g); err != nil {
					sklog.Fatalf("Could not abandon %d: %s", i.Issue, err)
				}
				fmt.Printf("Abandoned %d\n", i.Issue)
				break
			case 'n':
				fmt.Printf("Not abandoning %d\n", i.Issue)
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
