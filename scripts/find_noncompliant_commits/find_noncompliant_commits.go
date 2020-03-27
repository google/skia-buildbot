// Use to check that a string (i.e. email address) is present in the
// committer, author, or reviewer.

package main

import (
	"flag"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strings"
)

const (
	// We use double-newline between commits, since  asingle commit may have a
	// newline due to multiple reviewers.
	kGitLogFormat = "--pretty=%ce%n%ae%n%(trailers:key=Reviewed-by,valueonly)%n%n"
)

var (
	required_account = flag.String("required_string", "@google.com", "String that should be present, commits lacking this will be reported.")
	exclude_pattern  = flag.String("exclude_pattern", "autoroll|create-skp", "Regex for commiter/author/reviewer that will be bucketed separately if no google address is present.")
	git_since        = flag.String("git_since", "1 year ago", "string date or relative time passed to git log --since")

	reviewer_re = regexp.MustCompile("<(.*)>")
)

// getAccountSet will convert a newline separated list of emails into a set of
// emails present and will sanitize lines of the form
// 'Some Thing <sthing@example.com>' to just the email.
func getAccountSet(commit string) map[string]bool {
	accounts := make(map[string]bool)
	for _, account := range strings.Split(commit, "\n") {
		// Reviewers lines include full name, strip to email for consistency.
		stripped_account_name := reviewer_re.FindStringSubmatch(account)
		if len(stripped_account_name) > 0 {
			account = stripped_account_name[1]
		}
		accounts[account] = true
	}
	return accounts
}

func main() {
	flag.Parse()
	cmd := exec.Command("git", "--no-pager", "log", "--since", fmt.Sprintf(`"%s"`, *git_since), kGitLogFormat)
	fmt.Println(cmd.String())
	out, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}
	// Set of committer/author/reviewer combos that are problematic.
	bad_commits := make(map[string]int)
	// Count of required accounts associated with each commit.
	req_account_counts := make(map[int]int)
	// Number of times each non-required account is associated with a commit that
	// doesn't include a required account.
	non_required_account_freq := make(map[string]int)
	exclude_re := regexp.MustCompile(*exclude_pattern)
	for _, s := range strings.Split(string(out), "\n\n") {
		if s == "" {
			continue
		}
		num_required_accounts := strings.Count(s, *required_account)
		if num_required_accounts == 0 && exclude_re.MatchString(s) && *exclude_pattern != "" {
			req_account_counts[-1]++
			continue
		}
		req_account_counts[num_required_accounts]++
		if num_required_accounts == 0 {
			bad_commits[strings.ReplaceAll(s, "\n", "|")]++
			for acc := range getAccountSet(s) {
				non_required_account_freq[acc]++
			}
		}
	}

	fmt.Printf("\nCommits missing '%s' commiter/author/reviewer:\n", *required_account)
	for account_set, count := range bad_commits {
		fmt.Printf("%d times - %s\n", count, account_set)
	}
	fmt.Printf("\n\nCount of required accounts per commit (-1 is exclude_pattern matching commits):\n%v\n", req_account_counts)

	fmt.Println("\nAccounts on commits lacking required accounts:")
	for acc, count := range non_required_account_freq {
		fmt.Printf("%-40v%d\n", acc, count)
	}
}
