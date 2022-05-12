package main

import (
	"context"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/go/testutils/unittest"
	"google.golang.org/protobuf/encoding/prototext"
)

const testTemplate = `
{{ define "roller" }}
config {
    roller_name:  "{{.Name}}-chromium-m{{.Milestone.Milestone}}"
    child_bug_link:  "https://bugs.chromium.org/p/{{.Name}}/issues/entry"
    child_display_name:  "{{.Name}}"
    parent_bug_link:  "https://bugs.chromium.org/p/chromium/issues/entry"
    parent_display_name:  "Chromium m{{.Milestone.Milestone}}"
    parent_waterfall:  "https://build.chromium.org"
    owner_primary:  "borenet"
    owner_secondary:  "rmistry"
    contacts:  "skiabot@google.com"
    service_account:  "chromium-release-autoroll@skia-public.iam.gserviceaccount.com"
    reviewer:  ""
    commit_msg:  {
        bug_project:  "chromium"
        child_log_url_tmpl:  "{{.Repo}}/+log/{{` + "`" + `{{.RollingFrom}}` + "`" + `}}..{{` + "`" + `{{.RollingTo}}` + "`" + `}}"
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
        cpu:  "1"
        memory:  "2Gi"
        readiness_failure_threshold:  10
        readiness_initial_delay_seconds:  30
        readiness_period_seconds:  30
        image:  "TODO"
    }
    parent_child_repo_manager:  {
        gitiles_parent:  {
            gitiles:  {
                branch:  "{{.Milestone.Ref}}"
                repo_url:  "https://chromium.googlesource.com/chromium/src.git"
            }
            dep:  {
                primary:  {
                    id:  "{{.Repo}}"
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
                branch:  "refs/heads/chrome/m{{.Milestome.Milestone}}"
                repo_url:  "{{.Repo}}"
            }
        }
    }
}
{{ end }}

{{ range $index, $milestone := .Branches.ActiveMilestones }}
  {{ template "roller" map "Name" "skia" "Repo" "https://skia.googlesource.com/skia.git" "Milestone" $milestone }}
  {{ template "roller" map "Name" "angle" "Repo" "https://chromium.googlesource.com/angle.git" "Milestone" $milestone }}
{{ end }}
`

func TestProcess(t *testing.T) {
	unittest.LargeTest(t)

	// Setup.
	ctx := context.Background()
	tmp, err := ioutil.TempDir("", "autoroll-meta-config-generator-test")
	require.NoError(t, err)
	defer func() {
		require.NoError(t, os.RemoveAll(tmp))
	}()
	srcDir := filepath.Join(tmp, "src")
	relPath := filepath.Join("skia-public", "test.tmpl")
	srcFullPath := filepath.Join(srcDir, relPath)
	relDir, _ := filepath.Split(srcFullPath)
	dstDir := filepath.Join(tmp, "dst")
	require.NoError(t, os.MkdirAll(relDir, os.ModePerm))
	require.NoError(t, os.MkdirAll(dstDir, os.ModePerm))
	require.NoError(t, ioutil.WriteFile(srcFullPath, []byte(testTemplate), os.ModePerm))

	// Process the meta config.
	require.NoError(t, process(ctx, relPath, srcDir, dstDir, config_vars.FakeVars()))

	// Check the results.
	found := []string{}
	require.NoError(t, fs.WalkDir(os.DirFS(dstDir), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		found = append(found, path)
		return nil
	}))
	require.Equal(t, found, []string{
		filepath.Join("skia-public", "angle-chromium-m80.cfg"),
		filepath.Join("skia-public", "angle-chromium-m81.cfg"),
		filepath.Join("skia-public", "angle-chromium-m82.cfg"),
		filepath.Join("skia-public", "skia-chromium-m80.cfg"),
		filepath.Join("skia-public", "skia-chromium-m81.cfg"),
		filepath.Join("skia-public", "skia-chromium-m82.cfg"),
	})
	for _, path := range found {
		fullPath := filepath.Join(dstDir, path)
		configBytes, err := ioutil.ReadFile(fullPath)
		require.NoError(t, err)
		// Strip the first line from the config bytes (the header comment).
		configBytes = []byte(strings.Join(strings.Split(string(configBytes), "\n")[1:], "\n"))
		var cfg config.Config
		require.NoError(t, prototext.Unmarshal(configBytes, &cfg))
		require.NoError(t, cfg.Validate())
		rollerName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		require.Equal(t, cfg.RollerName, rollerName)
	}
}
