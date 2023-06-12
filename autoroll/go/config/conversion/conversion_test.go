package conversion

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSplitAndProcessPath(t *testing.T) {
	check := func(input, expectRelDir, expectBase string) {
		actualBase, actualRelDir := splitAndProcessPath(input)
		require.Equal(t, expectBase, actualBase)
		require.Equal(t, expectRelDir, actualRelDir)
	}
	check("path/to/roller.cfg", "path/to", "roller.cfg")
	check("path/to/generated/roller.cfg", "path/to", "roller.cfg")
	check("now/generated/in/middle/roller.cfg", "now/in/middle", "roller.cfg")
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
