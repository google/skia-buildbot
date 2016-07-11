package blacklist

import (
	"fmt"
	"io/ioutil"
	"path"
	"testing"

	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"

	assert "github.com/stretchr/testify/require"
)

func TestAddRemove(t *testing.T) {
	// Setup.
	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)
	assert.NoError(t, err)
	f := path.Join(tmp, "blacklist.json")
	b1, err := FromFile(f)
	assert.NoError(t, err)

	// Test.
	assert.Equal(t, len(DEFAULT_RULES), len(b1.Rules))
	r1 := &Rule{
		AddedBy:         "test@google.com",
		BuilderPatterns: []string{".*"},
		Name:            "My Rule",
	}
	assert.NoError(t, b1.addRule(r1))
	b2, err := FromFile(f)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, b1, b2)

	assert.NoError(t, b1.RemoveRule(r1.Name))
	b2, err = FromFile(f)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, b1, b2)
}

func TestRules(t *testing.T) {
	type testCase struct {
		builder     string
		commit      string
		expectMatch bool
		msg         string
	}
	tests := []struct {
		rule  Rule
		cases []testCase
	}{
		{
			rule: Rule{
				AddedBy: "test@google.com",
				Name:    "Match all builders",
				BuilderPatterns: []string{
					".*",
				},
			},
			cases: []testCase{
				testCase{
					builder:     "My-Builder",
					commit:      "abc123",
					expectMatch: true,
					msg:         "'.*' should match all builders.",
				},
			},
		},
		{
			rule: Rule{
				AddedBy: "test@google.com",
				Name:    "Match some builders",
				BuilderPatterns: []string{
					"My.*ilder",
				},
			},
			cases: []testCase{
				testCase{
					builder:     "My-Builder",
					commit:      "abc123",
					expectMatch: true,
					msg:         "Should match",
				},
				testCase{
					builder:     "Your-Builder",
					commit:      "abc123",
					expectMatch: false,
					msg:         "Should not match",
				},
			},
		},
		{
			rule: Rule{
				AddedBy: "test@google.com",
				Name:    "Match one commit",
				BuilderPatterns: []string{
					".*",
				},
				Commits: []string{
					"abc123",
				},
			},
			cases: []testCase{
				testCase{
					builder:     "My-Builder",
					commit:      "abc123",
					expectMatch: true,
					msg:         "Single commit match",
				},
				testCase{
					builder:     "My-Builder",
					commit:      "def456",
					expectMatch: false,
					msg:         "Single commit does not match",
				},
			},
		},
		{
			rule: Rule{
				AddedBy:         "test@google.com",
				Name:            "Commit range",
				BuilderPatterns: []string{},
				Commits: []string{
					"abc123",
					"bbadbeef",
				},
			},
			cases: []testCase{
				testCase{
					builder:     "My-Builder",
					commit:      "abc123",
					expectMatch: true,
					msg:         "Inside commit range (1)",
				},
				testCase{
					builder:     "My-Builder",
					commit:      "bbadbeef",
					expectMatch: true,
					msg:         "Inside commit range (2)",
				},
				testCase{
					builder:     "My-Builder",
					commit:      "def456",
					expectMatch: false,
					msg:         "Outside commit range",
				},
			},
		},
	}
	for _, test := range tests {
		for _, c := range test.cases {
			assert.Equal(t, c.expectMatch, test.rule.Match(c.builder, c.commit), c.msg)
		}
	}
}

func TestValidation(t *testing.T) {
	// Setup.
	tr := util.NewTempRepo()
	defer tr.Cleanup()
	remote := path.Join(tr.Dir, "skia.git")
	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)
	repos := gitinfo.NewRepoMap(tmp)
	_, err = repos.Repo(remote)
	assert.NoError(t, err)

	// Test.
	tests := []struct {
		rule   Rule
		expect error
		msg    string
	}{
		{
			rule: Rule{
				AddedBy:         "",
				Name:            "My rule",
				BuilderPatterns: []string{".*"},
				Commits:         []string{},
			},
			expect: fmt.Errorf("Rules must have an AddedBy user."),
			msg:    "No AddedBy",
		},
		{
			rule: Rule{
				AddedBy:         "test@google.com",
				Name:            "",
				BuilderPatterns: []string{".*"},
				Commits:         []string{},
			},
			expect: fmt.Errorf("Rules must have a name."),
			msg:    "No Name",
		},
		{
			rule: Rule{
				AddedBy:         "test@google.com",
				Name:            "01234567890123456789012345678901234567890123456789",
				BuilderPatterns: []string{".*"},
				Commits:         []string{},
			},
			expect: nil,
			msg:    "Long Name",
		},
		{
			rule: Rule{
				AddedBy:         "test@google.com",
				Name:            "012345678901234567890123456789012345678901234567890",
				BuilderPatterns: []string{".*"},
				Commits:         []string{},
			},
			expect: fmt.Errorf("Rule names must be shorter than 50 characters. Use the Description field for detailed information."),
			msg:    "Too Long Name",
		},
		{
			rule: Rule{
				AddedBy:         "test@google.com",
				Name:            "My rule",
				BuilderPatterns: []string{},
				Commits:         []string{},
			},
			expect: fmt.Errorf("Rules must include a builder pattern and/or a commit/range."),
			msg:    "No builders or commits",
		},
		{
			rule: Rule{
				AddedBy:         "test@google.com",
				Name:            "My rule",
				BuilderPatterns: []string{".*"},
				Commits:         []string{},
			},
			expect: nil,
			msg:    "One builder pattern, no commits",
		},
		{
			rule: Rule{
				AddedBy: "test@google.com",
				Name:    "My rule",
				BuilderPatterns: []string{
					"A.*B",
					"[C|D]{42}",
					"[Your|My]-Builder",
					"Test.*",
					"Some-Builder",
				},
				Commits: []string{},
			},
			expect: nil,
			msg:    "Five builder patterns, no commits",
		},
		{
			rule: Rule{
				AddedBy:         "test@google.com",
				Name:            "My rule",
				BuilderPatterns: []string{},
				Commits: []string{
					"06eb2a58139d3ff764f10232d5c8f9362d55e20f",
				},
			},
			expect: nil,
			msg:    "One commit",
		},
		{
			rule: Rule{
				AddedBy:         "test@google.com",
				Name:            "My rule",
				BuilderPatterns: []string{},
				Commits: []string{
					"06eb2a58139d3ff764f10232d5c8f9362d55",
				},
			},
			expect: fmt.Errorf("%q is not a valid commit.", "06eb2a58139d3ff764f10232d5c8f9362d55"),
			msg:    "Invalid commit",
		},
		{
			rule: Rule{
				AddedBy:         "test@google.com",
				Name:            "My rule",
				BuilderPatterns: []string{},
				Commits: []string{
					"051955c355eb742550ddde4eccc3e90b6dc5b887",
					"4b822ebb7cedd90acbac6a45b897438746973a87",
					"d74dfd42a48325ab2f3d4a97278fc283036e0ea4",
					"6d4811eddfa637fac0852c3a0801b773be1f260d",
					"67635e7015d74b06c00154f7061987f426349d9f",
				},
			},
			expect: nil,
			msg:    "Five commits",
		},
	}
	for _, test := range tests {
		assert.Equal(t, test.expect, ValidateRule(&test.rule, repos), test.msg)
	}
}

func TestCommitRange(t *testing.T) {
	// Setup.
	tr := util.NewTempRepo()
	defer tr.Cleanup()
	remote := path.Join(tr.Dir, "skia.git")
	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)
	repos := gitinfo.NewRepoMap(tmp)
	_, err = repos.Repo(remote)
	assert.NoError(t, err)
	f := path.Join(tmp, "blacklist.json")
	b, err := FromFile(f)
	assert.NoError(t, err)

	// Test.

	// Create a commit range rule.
	startCommit := "051955c355eb742550ddde4eccc3e90b6dc5b887"
	endCommit := "d30286d2254716d396073c177a754f9e152bbb52"
	rule, err := NewCommitRangeRule("commit range", "test@google.com", "...", []string{}, startCommit, endCommit, repos)
	assert.NoError(t, err)
	err = b.AddRule(rule, repos)
	assert.NoError(t, err)

	// Ensure that we got the expected list of commits.
	assert.Equal(t, []string{
		"8d2d1247ef5d2b8a8d3394543df6c12a85881296",
		"4b822ebb7cedd90acbac6a45b897438746973a87",
		"051955c355eb742550ddde4eccc3e90b6dc5b887",
	}, b.Rules["commit range"].Commits)

	// Test a few commits.
	tc := []struct {
		commit string
		expect bool
		msg    string
	}{
		{
			commit: "",
			expect: false,
			msg:    "empty commit does not match",
		},
		{
			commit: startCommit,
			expect: true,
			msg:    "startCommit matches",
		},
		{
			commit: endCommit,
			expect: false,
			msg:    "endCommit does not match",
		},
		{
			commit: "4b822ebb7cedd90acbac6a45b897438746973a87",
			expect: true,
			msg:    "middle of range matches",
		},
		{
			commit: "06eb2a58139d3ff764f10232d5c8f9362d55e20f",
			expect: false,
			msg:    "out of range does not match",
		},
	}
	for _, c := range tc {
		assert.Equal(t, c.expect, b.Match("", c.commit))
	}
}

func TestNoTrybots(t *testing.T) {
	// Setup.
	tr := util.NewTempRepo()
	defer tr.Cleanup()
	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)
	f := path.Join(tmp, "blacklist.json")
	b, err := FromFile(f)
	assert.NoError(t, err)

	// Test.
	assert.True(t, b.Match("Test-Android-GCC-Nexus7-GPU-Tegra3-Arm7-Release-Trybot", ""))

	assert.Equal(t, "Cannot remove built-in rule \"Trybots\"", b.RemoveRule("Trybots").Error())
}
