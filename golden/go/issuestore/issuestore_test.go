package issuestore

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
)

const (
	TEST_DATA_DIR = "./testdata"

	// Prefixes for different generated item types.
	ISSUE_PREFIX   = "ISSUES_"
	DIGEST_PREFIX  = "DIGEST_"
	IGNORES_PREFIX = "IGNORES_"
	TRACE_PREFIX   = "TRACE_"
	TEST_PREFIX    = "TEST_"
)

var (
	ISSUE_1, ISSUE_2, ISSUE_3 = "3333", "3334", "3335"
	DIG_1, DIG_2, DIG_3       = "ABC", "EFG", "HIJ"

	INIT_ISSUES = []*Rec{
		&Rec{
			IssueID:   ISSUE_1,
			Digests:   []string{},
			Traces:    []string{},
			Ignores:   []string{},
			TestNames: []string{},
		},
	}
)

func TestIssueStore(t *testing.T) {
	const N_ISSUES = 20

	// Add a number of issues
	issueStore, err := New(TEST_DATA_DIR)
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, TEST_DATA_DIR)

	lookup := map[string][]string{}
	initIssues := genIssues(t, lookup, N_ISSUES, N_ISSUES/4, N_ISSUES/2, N_ISSUES/3, N_ISSUES/3)
	issueIDs := make([]string, 0, len(initIssues))
	for _, issue := range initIssues {
		assert.NoError(t, issueStore.Add(issue))
		issueIDs = append(issueIDs, issue.IssueID)
	}

	for _, issue := range initIssues {
		found, err := issueStore.Get([]string{issue.IssueID})
		assert.NoError(t, err)
		assert.Equal(t, 1, len(found))
		assert.Equal(t, issue, found[0])
	}

	found, err := issueStore.Get(issueIDs)
	assert.NoError(t, err)
	assert.Equal(t, initIssues, found)

	testAgainstLookup(t, issueStore, lookup)

	// // Assert them I can read them by issue id, digest, traceid and testNames
	updateIssues := genIssues(t, lookup, N_ISSUES, N_ISSUES/4, N_ISSUES/4, N_ISSUES/4, N_ISSUES/4)
	for _, issue := range updateIssues {
		assert.NoError(t, issueStore.Add(issue))
	}
	testAgainstLookup(t, issueStore, lookup)

	for idx, issue := range updateIssues {
		initIssues[idx].Merge(issue)
	}

	// Test the list function.
	for i := 0; i < N_ISSUES; i += 2 {
		foundList, total, err := issueStore.List(i, 2)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(foundList))
		assert.Equal(t, N_ISSUES, total)
		assert.Equal(t, initIssues[i], foundList[0])
		assert.Equal(t, initIssues[i+1], foundList[1])
	}

	foundList, total, err := issueStore.List(0, N_ISSUES+5)
	assert.NoError(t, err)
	assert.Equal(t, N_ISSUES, len(foundList))
	assert.Equal(t, N_ISSUES, total)
	assert.Equal(t, initIssues, foundList)

	// Delete some entries

	// Delete records

	// Make sure they are empty

}

func testAgainstLookup(t assert.TestingT, issueStore IssueStore, lookup map[string][]string) {
	for itemID, exp := range lookup {
		var found []string
		var err error
		if strings.HasPrefix(itemID, DIGEST_PREFIX) {
			found, err = issueStore.ByDigest(itemID)
		} else if strings.HasPrefix(itemID, IGNORES_PREFIX) {
			found, err = issueStore.ByIgnore(itemID)
		} else if strings.HasPrefix(itemID, TRACE_PREFIX) {
			found, err = issueStore.ByTrace(itemID)
		} else if strings.HasPrefix(itemID, TEST_PREFIX) {
			found, err = issueStore.ByTest(itemID)
		} else {
			t.FailNow()
		}
		assert.NoError(t, err)
		assert.Equal(t, exp, found)
	}
}

func genIssues(t *testing.T, lookup map[string][]string, nIssues int, nDigests int, nTraces int, nIgnores int, nTestNames int) []*Rec {
	// generate a list of issues and the given number of digests/traces and tests.
	issues := fmtStrings(ISSUE_PREFIX+"%d", nIssues)
	digests := fmtStrings(DIGEST_PREFIX+"%d", 5*nDigests)
	ignores := fmtStrings(IGNORES_PREFIX+"%d", 3*nIgnores)
	traces := fmtStrings(TRACE_PREFIX+"%d", 5*nTraces)
	testNames := fmtStrings(TEST_PREFIX+"%d", 5*nTestNames)

	ret := make([]*Rec, nIssues)
	for idx, IssueID := range issues {
		r := &Rec{
			IssueID:   IssueID,
			Digests:   drawN(digests, nDigests),
			Traces:    drawN(traces, nTraces),
			Ignores:   drawN(ignores, nIgnores),
			TestNames: drawN(testNames, nTestNames),
		}
		assert.Equal(t, []int{nDigests, nTraces, nIgnores, nTestNames}, []int{len(r.Digests), len(r.Traces), len(r.Ignores), len(r.TestNames)})
		addLookup(lookup, r)
		ret[idx] = r
	}
	return ret
}

func addLookup(lookup map[string][]string, rec *Rec) {
	addLookupItem(lookup, rec.Digests, rec.IssueID)
	addLookupItem(lookup, rec.Traces, rec.IssueID)
	addLookupItem(lookup, rec.Ignores, rec.IssueID)
	addLookupItem(lookup, rec.TestNames, rec.IssueID)
}

func addLookupItem(lookup map[string][]string, ids []string, parentID string) {
	for _, id := range ids {
		lookup[id] = util.NewStringSet(lookup[id], []string{parentID}).Keys()
		sort.Strings(lookup[id])
	}
}

func fmtStrings(template string, n int) []string {
	ret := make([]string, n)
	for i := 0; i < n; i++ {
		ret[i] = fmt.Sprintf(template, i)
	}
	return ret
}

func drawN(strs []string, n int) []string {
	indices := rand.Perm(len(strs))
	ret := make([]string, n)
	for i := 0; i < n; i++ {
		ret[i] = strs[indices[i]]
	}
	return ret
}
