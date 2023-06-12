package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/go/testutils"
)

const fakeTmplContents = `
{{ define "roller" }}
config {
    roller_name:  "gen-roller-{{sanitize .Milestone.V8Branch}}-chromium-m{{.Milestone.Milestone}}"
    child_display_name:  "gen-roller {{.Milestone.V8Branch}}"
    parent_display_name:  "Chromium m{{.Milestone.Milestone}}"
    parent_waterfall:  "blah blah"
    owner_primary:  "me"
    owner_secondary:  "you"
    contacts:  "me@google.com"
    contacts:  "you@google.com"
    service_account:  "my-fake@account.com"
    reviewer:  "me@google.com"
    commit_msg:  {
        include_log:  true
        include_revision_count:  true
        include_tbr_line:  true
        built_in:  DEFAULT
    }
    gerrit:  {
        url:  "https://chromium-review.googlesource.com"
        project:  "chromium/src"
        config:  CHROMIUM_BOT_COMMIT
    }
    kubernetes:  {
        cpu:  "0.1"
        memory:  "2Gi"
        readiness_failure_threshold:  10
        readiness_initial_delay_seconds:  30
        readiness_period_seconds:  30
        image:  "gcr.io/skia-public/autoroll-be@sha256:e5b65806a089505d7b8e8351c01e50c10c2f941d4cd966cdc022a072391e4f0b"
    }
    parent_child_repo_manager:  {
        gitiles_parent:  {
            gitiles:  {
                branch:  "{{.Milestone.Ref}}"
                repo_url:  "https://chromium.googlesource.com/chromium/src.git"
            }
            dep:  {
                primary:  {
                    id:  "https://chromium.googlesource.com/gen-roller/gen-roller.git"
                    path:  "DEPS"
                }
            }
            gerrit:  {
                url:  "https://chromium-review.googlesource.com"
                project:  "chromium/src"
                config:  CHROMIUM_BOT_COMMIT
            }
        }
        gitiles_child:  {
            gitiles:  {
                branch:  "refs/heads/{{.Milestone.V8Branch}}-lkgr"
                repo_url:  "https://chromium.googlesource.com/gen-roller/gen-roller.git"
            }
        }
    }
}
{{ end }}

{{ range $index, $milestone := .Branches.ActiveMilestones }}
  {{ template "roller" map "Milestone" $milestone "Index" $index }}
{{ end }}
`

const multipleConfigs = `
config {
    roller_name:  "gen-roller-11-5-chromium-m115"
}
config {
    roller_name:  "gen-roller-11-4-chromium-m114"
}
config {
    roller_name:  "gen-roller-10-8-chromium-m108"
}
`

func TestSplitConfigs(t *testing.T) {
	matches := configStartRegex.FindAllString(multipleConfigs, -1)
	require.Len(t, matches, 3)
	configs := splitConfigs([]byte(multipleConfigs))
	for idx, config := range configs {
		configs[idx] = bytes.TrimSpace(config)
	}
	require.Equal(t, [][]byte{
		[]byte(`roller_name:  "gen-roller-11-5-chromium-m115"`),
		[]byte(`roller_name:  "gen-roller-11-4-chromium-m114"`),
		[]byte(`roller_name:  "gen-roller-10-8-chromium-m108"`),
	}, configs)
}

func TestRollerNameRegex(t *testing.T) {
	matches := rollerNameRegex.FindSubmatch([]byte(`config {
		roller_name:  "gen-roller-10-8-chromium-m108"
}`))
	require.Len(t, matches, 2)
	require.Equal(t, matches[1], []byte("gen-roller-10-8-chromium-m108"))
}

// TestProcessTemplate just tests a simple config template with mocked data sources.
func TestProcessTemplate(t *testing.T) {
	vars := &templateVars{
		Vars: config_vars.FakeVars(),
	}
	tmp, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)
	dir := filepath.Join(tmp, "templates")
	require.NoError(t, os.MkdirAll(dir, os.ModePerm))
	file := filepath.Join(dir, "my-template.tmpl")
	require.NoError(t, ioutil.WriteFile(file, []byte(fakeTmplContents), os.ModePerm))
	generatedDir := filepath.Join(tmp, "generated")

	actual, err := processTemplate(file, vars)
	require.NoError(t, err)
	require.Len(t, actual, 3)
	actual1, ok := actual[generatedDir+"/gen-roller-8-0-chromium-m80.cfg"]
	require.True(t, ok)
	require.NotNil(t, actual1)
	actual2, ok := actual[generatedDir+"/gen-roller-8-1-chromium-m81.cfg"]
	require.True(t, ok)
	require.NotNil(t, actual2)
	actual3, ok := actual[generatedDir+"/gen-roller-8-2-chromium-m82.cfg"]
	require.True(t, ok)
	require.NotNil(t, actual3)
}

func TestSortPrivacySandboxVersionSlice(t *testing.T) {
	// Check inequality.
	checkLess := func(less, more *privacySandboxVersion) {
		s := privacySandboxVersionSlice([]*privacySandboxVersion{less, more})
		require.True(t, s.Less(0, 1))
		require.False(t, s.Less(1, 0))
		require.False(t, s.Less(0, 0))
		require.False(t, s.Less(1, 1))
	}
	checkLess(&privacySandboxVersion{
		BranchName: "less",
	}, &privacySandboxVersion{
		BranchName: "more",
	})
	checkLess(&privacySandboxVersion{
		Ref: "less",
	}, &privacySandboxVersion{
		Ref: "more",
	})
	checkLess(&privacySandboxVersion{
		Bucket: "less",
	}, &privacySandboxVersion{
		Bucket: "more",
	})
	checkLess(&privacySandboxVersion{
		PylFile: "less",
	}, &privacySandboxVersion{
		PylFile: "more",
	})
	checkLess(&privacySandboxVersion{
		PylTargetPath: "less",
	}, &privacySandboxVersion{
		PylTargetPath: "more",
	})
	checkLess(&privacySandboxVersion{
		CipdPackage: "less",
	}, &privacySandboxVersion{
		CipdPackage: "more",
	})
	checkLess(&privacySandboxVersion{
		CipdTag: "less",
	}, &privacySandboxVersion{
		CipdTag: "more",
	})

	// Check equality.
	s := privacySandboxVersionSlice([]*privacySandboxVersion{{}, {}})
	require.False(t, s.Less(0, 1))
	require.False(t, s.Less(1, 0))
	require.False(t, s.Less(0, 0))
	require.False(t, s.Less(1, 1))
}

func TestSanitize(t *testing.T) {
	test := func(input, expect string) {
		actual := sanitize(input)
		require.Equal(t, expect, actual)
	}

	test("my-name", "my-name")
	test("my--name", "my-name")
	test("my-name-", "my-name-")
	test("my.name", "my-name")
	test(".../...", "-")
}
