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
	// Add a number of issues
	issueStore, err := New(TEST_DATA_DIR)
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, TEST_DATA_DIR)

	initIssues, lookup := genIssues(t, 5, 3, 10, 6, 3)
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
	return

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
			t.Fail()
		}
		assert.NoError(t, err)
		assert.Equal(t, exp, found)
	}

	// // Assert them I can read them by issue id, digest, traceid and testNames

	// byDigest, err := issueStore.ByDigest("")
	// assert.NoError(t, err)
	// assert.True(t, false)

	// Add setting

	// Delete some entries

	// Delete records

	// Make sure they are empty

}

func genIssues(t *testing.T, nIssues int, nDigests int, nTraces int, nIgnores int, nTestNames int) ([]*Rec, map[string][]string) {
	// generate a list of issues and the given number of digests/traces and tests.
	issues := fmtStrings(ISSUE_PREFIX+"%d", nIssues)
	digests := fmtStrings(DIGEST_PREFIX+"%d", 5*nDigests)
	ignores := fmtStrings(IGNORES_PREFIX+"%d", 3*nIgnores)
	traces := fmtStrings(TRACE_PREFIX+"%d", 5*nTraces)
	testNames := fmtStrings(TEST_PREFIX+"%d", 5*nTestNames)

	ret := make([]*Rec, nIssues)
	lookup := map[string][]string{}
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
	return ret, lookup
}

func addLookup(lookup map[string][]string, rec *Rec) {
	addLookupItem(lookup, rec.Digests, rec.IssueID)
	addLookupItem(lookup, rec.Traces, rec.IssueID)
	addLookupItem(lookup, rec.Ignores, rec.IssueID)
	addLookupItem(lookup, rec.TestNames, rec.IssueID)
}

func addLookupItem(lookup map[string][]string, ids []string, parentId string) {
	for _, id := range ids {
		lookup[id] = util.NewStringSet(lookup[id], []string{id}).Keys()
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
