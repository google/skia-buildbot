package conversion

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"google.golang.org/protobuf/encoding/prototext"
)

func TestSortPrivacySandboxVersionSlice(t *testing.T) {
	// Check inequality.
	checkLess := func(less, more *PrivacySandboxVersion) {
		s := PrivacySandboxVersionSlice([]*PrivacySandboxVersion{less, more})
		require.True(t, s.Less(0, 1))
		require.False(t, s.Less(1, 0))
		require.False(t, s.Less(0, 0))
		require.False(t, s.Less(1, 1))
	}
	checkLess(&PrivacySandboxVersion{
		BranchName: "less",
	}, &PrivacySandboxVersion{
		BranchName: "more",
	})
	checkLess(&PrivacySandboxVersion{
		Ref: "less",
	}, &PrivacySandboxVersion{
		Ref: "more",
	})
	checkLess(&PrivacySandboxVersion{
		Bucket: "less",
	}, &PrivacySandboxVersion{
		Bucket: "more",
	})
	checkLess(&PrivacySandboxVersion{
		PylFile: "less",
	}, &PrivacySandboxVersion{
		PylFile: "more",
	})
	checkLess(&PrivacySandboxVersion{
		PylTargetPath: "less",
	}, &PrivacySandboxVersion{
		PylTargetPath: "more",
	})
	checkLess(&PrivacySandboxVersion{
		CipdPackage: "less",
	}, &PrivacySandboxVersion{
		CipdPackage: "more",
	})
	checkLess(&PrivacySandboxVersion{
		CipdTag: "less",
	}, &PrivacySandboxVersion{
		CipdTag: "more",
	})

	// Check equality.
	s := PrivacySandboxVersionSlice([]*PrivacySandboxVersion{{}, {}})
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

// TestProcessTemplate just tests a simple config template with mocked data sources.
func TestProcessTemplate(t *testing.T) {
	ctx := context.Background()
	vars := &TemplateVars{
		Vars: config_vars.FakeVars(),
	}
	actual, err := ProcessTemplate(
		ctx,
		nil, // client is only used if checkGCSArtifacts is true
		"my-cluster/templates/my-template.tmpl",
		fakeTmplContents,
		vars,
		false, // We don't want to hit GCS (or bother mocking requests)
	)
	require.NoError(t, err)
	require.Len(t, actual, 3)
	actual1, ok := actual["my-cluster/gen-roller-8-0-chromium-m80.cfg"]
	require.True(t, ok)
	require.NotNil(t, actual1)
	actual2, ok := actual["my-cluster/gen-roller-8-1-chromium-m81.cfg"]
	require.True(t, ok)
	require.NotNil(t, actual2)
	actual3, ok := actual["my-cluster/gen-roller-8-2-chromium-m82.cfg"]
	require.True(t, ok)
	require.NotNil(t, actual3)

	for _, cfgBytes := range [][]byte{actual1, actual2, actual3} {
		var cfg config.Config
		require.NoError(t, prototext.Unmarshal(cfgBytes, &cfg))
		require.NoError(t, cfg.Validate())
	}
}

const fakeTmplContents = `
{{ define "roller" }}
config {
    roller_name:  "gen-roller-{{sanitize .Milestone.V8Branch}}-chromium-m{{.Milestone.Milestone}}"
    child_display_name:  "v8 {{.Milestone.V8Branch}}"
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
                    id:  "https://chromium.googlesource.com/v8/v8.git"
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
                repo_url:  "https://chromium.googlesource.com/v8/v8.git"
            }
        }
    }
}
{{ end }}

{{ range $index, $milestone := .Branches.ActiveMilestones }}
  {{ template "roller" map "Milestone" $milestone "Index" $index }}
{{ end }}
`
