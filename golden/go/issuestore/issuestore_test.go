package issuestore

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
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

func TestIssueStore(t *testing.T) {
	// Medium test because it stores a bolt db to disk
	unittest.MediumTest(t)
	const N_ISSUES = 20

	// Add a number of issues
	issueStore, err := New(TEST_DATA_DIR)
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, TEST_DATA_DIR)

	lookup := map[string][]string{}
	initIssues := genIssues(t, lookup, N_ISSUES, N_ISSUES/4+1, N_ISSUES/2+1, N_ISSUES/3+1, N_ISSUES/3+1)
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
	updateIssues := genIssues(t, lookup, N_ISSUES/2+1, N_ISSUES/4+1, N_ISSUES/4+1, N_ISSUES/4+1, N_ISSUES/4+1)
	updatedIssuesIDs := []string{}
	for idx, issue := range updateIssues {
		assert.NoError(t, issueStore.Add(issue))
		initIssues[idx].Add(issue)
		updatedIssuesIDs = append(updatedIssuesIDs, issue.IssueID)
	}
	assert.Len(t, updatedIssuesIDs, 11)
	// Do a spot check
	assert.Contains(t, updatedIssuesIDs, "ISSUES_0003")
	testAgainstLookup(t, issueStore, lookup)

	// Test the list function.
	for i := 0; i < N_ISSUES; i += 2 {
		foundList, total, err := issueStore.List(i, 2)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(foundList))
		assert.Equal(t, N_ISSUES, total)
		compareEntries(t, initIssues[i:i+1], foundList[0:1])
		compareEntries(t, initIssues[i+1:i+2], foundList[1:2])
	}

	foundList, total, err := issueStore.List(0, N_ISSUES+5)
	assert.NoError(t, err)
	assert.Equal(t, N_ISSUES, len(foundList))
	assert.Equal(t, N_ISSUES, total)
	compareEntries(t, initIssues, foundList)

	// Remove the previously added entries.
	for idx, issue := range updateIssues {
		assert.NoError(t, issueStore.Subtract(issue))
		removeLookup(lookup, issue)
		initIssues[idx].Subtract(issue)
		found, err := issueStore.Get([]string{issue.IssueID})
		assert.NoError(t, err)
		compareEntries(t, initIssues[idx:idx+1], found[0:1])
	}

	testAgainstLookup(t, issueStore, lookup)
	foundList, total, err = issueStore.List(0, -1)
	assert.NoError(t, err)
	assert.Equal(t, N_ISSUES, total)
	compareEntries(t, foundList, initIssues)

	// Remove all entries to check at the bottom whether indices have been removed.
	for _, issue := range initIssues {
		removeLookup(lookup, issue)
	}

	// Subtract all annotations of a subset of issues.
	for _, issue := range initIssues[:len(updateIssues)] {
		assert.NoError(t, issueStore.Subtract(issue))
	}
	initIssues = initIssues[len(updateIssues):]
	foundList, total, err = issueStore.List(0, -1)
	assert.NoError(t, err)
	// Should be 9 now because we started with 20 and removed 11.
	assert.Equal(t, 9, total)
	compareEntries(t, foundList, initIssues)

	// Delete all issues and make sure they are gone.
	assert.NoError(t, issueStore.Delete(issueIDs))
	foundList, total, err = issueStore.List(0, -1)
	assert.NoError(t, err)
	assert.Equal(t, []*Annotation{}, foundList)
	assert.Equal(t, 0, total)
	testAgainstLookup(t, issueStore, lookup)
}

func compareEntries(t assert.TestingT, exps []*Annotation, actual []*Annotation) {
	assert.Equal(t, len(exps), len(actual))
	for i, exp := range exps {
		assert.Equal(t, exp.IssueID, actual[i].IssueID)
		compareList(t, exp.Digests, actual[i].Digests)
		compareList(t, exp.Traces, actual[i].Traces)
		compareList(t, exp.Ignores, actual[i].Ignores)
		compareList(t, exp.TestNames, actual[i].TestNames)
	}
}

func compareList(t assert.TestingT, exp, actual []string) {
	sort.Strings(exp)
	sort.Strings(actual)
	assert.Equal(t, exp, actual)
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

func genIssues(t *testing.T, lookup map[string][]string, nIssues int, nDigests int, nTraces int, nIgnores int, nTestNames int) []*Annotation {
	// generate a list of issues and the given number of digests/traces and tests.
	issues := fmtStrings(ISSUE_PREFIX+"%04d", nIssues)
	digests := fmtStrings(DIGEST_PREFIX+"%04d", 5*nDigests)
	ignores := fmtStrings(IGNORES_PREFIX+"%04d", 3*nIgnores)
	traces := fmtStrings(TRACE_PREFIX+"%04d", 5*nTraces)
	testNames := fmtStrings(TEST_PREFIX+"%04d", 5*nTestNames)

	ret := make([]*Annotation, nIssues)
	for idx, issueID := range issues {
		r := &Annotation{
			IssueID:   issueID,
			Digests:   drawN(digests, nDigests, lookup, issueID),
			Traces:    drawN(traces, nTraces, lookup, issueID),
			Ignores:   drawN(ignores, nIgnores, lookup, issueID),
			TestNames: drawN(testNames, nTestNames, lookup, issueID),
		}
		assert.Equal(t, []int{nDigests, nTraces, nIgnores, nTestNames}, []int{len(r.Digests), len(r.Traces), len(r.Ignores), len(r.TestNames)})
		addLookup(lookup, r)
		ret[idx] = r
	}
	return ret
}

func addLookup(lookup map[string][]string, rec *Annotation) {
	addLookupItem(lookup, rec.Digests, rec.IssueID)
	addLookupItem(lookup, rec.Traces, rec.IssueID)
	addLookupItem(lookup, rec.Ignores, rec.IssueID)
	addLookupItem(lookup, rec.TestNames, rec.IssueID)
}

func removeLookup(lookup map[string][]string, delta *Annotation) {
	removeLookupItem(lookup, delta.Digests, delta.IssueID)
	removeLookupItem(lookup, delta.Traces, delta.IssueID)
	removeLookupItem(lookup, delta.Ignores, delta.IssueID)
	removeLookupItem(lookup, delta.TestNames, delta.IssueID)
}

func addLookupItem(lookup map[string][]string, ids []string, parentID string) {
	for _, id := range ids {
		lookup[id] = util.NewStringSet(lookup[id], []string{parentID}).Keys()
		sort.Strings(lookup[id])
	}
}

func removeLookupItem(lookup map[string][]string, ids []string, parentID string) {
	for _, id := range ids {
		s := util.NewStringSet(lookup[id])
		delete(s, parentID)
		lookup[id] = s.Keys()
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

func drawN(strs []string, n int, lookup map[string][]string, ignoreParent string) []string {
	indices := rand.Perm(len(strs))
	ret := make([]string, 0, n)
	for i := 0; (i < len(indices)) && (len(ret) < n); i++ {
		str := strs[indices[i]]
		if ignoreParent == "" || !util.In(ignoreParent, lookup[str]) {
			ret = append(ret, strs[indices[i]])
		}
	}
	return ret
}
