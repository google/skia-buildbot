package main

import (
	"context"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/go/gitiles/mocks"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"google.golang.org/protobuf/encoding/prototext"
)

const (
	testTemplate = `
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
{{ end }}
`

	staticTemplate = `config {
	roller_name:  "skia-skiabot-test-autoroll"
	child_bug_link:  "https://bugs.chromium.org/p/skia/issues/entry"
	child_display_name:  "Skia"
	parent_display_name:  "Skiabot Test"
	parent_waterfall:  "https://status-staging.skia.org/repo/skiabot-test"
	owner_primary:  "borenet"
	owner_secondary:  "rmistry"
	contacts:  "borenet@google.com"
	service_account:  "skia-autoroll@skia-public.iam.gserviceaccount.com"
	reviewer:  "borenet@google.com"
	supports_manual_rolls:  true
	commit_msg:  {
	  child_log_url_tmpl:  "https://skia.googlesource.com/skia.git/+log/{{` + "`" + `{{.RollingFrom}}` + "`" + `}}..{{` + "`" + `{{.RollingTo}}` + "`" + `}}"
	  include_log:  true
	  include_revision_count:  true
	  include_tbr_line:  true
	  include_tests:  true
	  built_in:  DEFAULT
	}
	gerrit:  {
	  url:  "https://skia-review.googlesource.com"
	  project:  "skiabot-test"
	  config:  CHROMIUM_BOT_COMMIT
	}
	kubernetes:  {
	  cpu:  "1"
	  memory:  "2Gi"
	  readiness_failure_threshold:  10
	  readiness_initial_delay_seconds:  30
	  readiness_period_seconds:  30
	  image:  "gcr.io/skia-public/autoroll-be@sha256:4b4842f020a993e7fa2458e83ece11f51d1906f65ced4706a37eeb06f7ac11c9"
	}
	parent_child_repo_manager:  {
	  gitiles_parent:  {
		gitiles:  {
		  branch:  "main"
		  repo_url:  "https://skia.googlesource.com/skiabot-test.git"
		}
		dep:  {
		  primary:  {
			id:  "https://skia.googlesource.com/skia.git"
			path:  "DEPS"
		  }
		}
		gerrit:  {
		  url:  "https://skia-review.googlesource.com"
		  project:  "skiabot-test"
		  config:  CHROMIUM_BOT_COMMIT
		}
	  }
	  gitiles_child:  {
		gitiles:  {
		  branch:  "main"
		  repo_url:  "https://skia.googlesource.com/skia.git"
		}
	  }
	}
	notifiers:  {
	  msg_type:  LAST_N_FAILED
	  monorail:  {
		project:  "skia"
		owner:  "borenet"
		cc:  "rmistry@google.com"
		components:  "AutoRoll"
	  }
	}
}
`
)

var expectedPaths = []string{
	"skia-public/skia-chromium-m80.cfg",
	"skia-public/skia-chromium-m81.cfg",
	"skia-public/skia-chromium-m82.cfg",
}

func checkResults(t *testing.T, expectedPaths []string, results map[string]string) {
	// Ensure that all expected paths are in the result set.
	require.Len(t, results, len(expectedPaths))
	for _, key := range expectedPaths {
		_, ok := results[key]
		require.True(t, ok)
	}

	// Ensure that the generated configs are parseable.
	for path, contents := range results {
		require.NotEmpty(t, contents, "Path %s is empty; was the file deleted?", path)
		// Strip the first two lines from the config bytes (the header comment).
		configBytes := []byte(strings.Join(strings.Split(contents, "\n")[2:], "\n"))
		var cfg config.Config
		require.NoError(t, prototext.Unmarshal(configBytes, &cfg))
		require.NoError(t, cfg.Validate())
		rollerName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		require.Equal(t, cfg.RollerName, rollerName)
	}
}

func TestProcess(t *testing.T) {
	unittest.SmallTest(t)

	// Setup.
	ctx := context.Background()
	relPath := path.Join("skia-public", "templates", "test.tmpl")
	dstDir := "skia-public"

	// Process the meta config.
	results, err := process(ctx, relPath, testTemplate, dstDir, config_vars.FakeVars())
	require.NoError(t, err)

	// Check the results.
	checkResults(t, expectedPaths, results)
}

func TestProcessDir(t *testing.T) {
	unittest.SmallTest(t)

	// Setup.
	ctx := context.Background()
	srcDir := "skia-public/templates"
	dstDir := "skia-public"
	vars := config_vars.FakeVars()
	mockRepo := &mocks.GitilesRepo{}
	mockCommit := "abc123"
	tmplBaseName := "test.tmpl"
	tmplPath := srcDir + "/" + tmplBaseName

	// Mock the interactions with Gitiles.
	oldConfigFile := "skia-public/skia-chromium-m79.cfg"
	mockPaths := append([]string{oldConfigFile}, expectedPaths[:len(expectedPaths)-1]...)
	for index := range mockPaths {
		mockPaths[index] = strings.SplitN(mockPaths[index], "/", 2)[1]
	}
	mockRepo.On("ListFilesRecursiveAtRef", testutils.AnyContext, dstDir, mockCommit).Return(mockPaths, nil)
	mockRepo.On("ListFilesRecursiveAtRef", testutils.AnyContext, srcDir, mockCommit).Return([]string{tmplBaseName}, nil)
	for _, path := range mockPaths {
		contents := []byte(fmt.Sprintf(generatedFileHeaderTmpl, tmplPath))
		mockRepo.On("ReadFileAtRef", testutils.AnyContext, strings.Join([]string{"skia-public", path}, "/"), mockCommit).Return(contents, nil)
	}
	// This config file doesn't exist yet.
	mockRepo.On("ReadFileAtRef", testutils.AnyContext, expectedPaths[len(expectedPaths)-1], mockCommit).Return(nil, errors.New("does not exist"))
	mockRepo.On("ReadFileAtRef", testutils.AnyContext, tmplPath, mockCommit).Return([]byte(testTemplate), nil)

	// Compute the results.
	results, err := processDir(ctx, "skia-public/templates", "skia-public", vars, mockRepo, mockCommit)
	require.NoError(t, err)

	// We should have deleted the old config file.
	got, ok := results[oldConfigFile]
	require.True(t, ok)
	require.Equal(t, got, "")
	// Delete the entry from the results; it's empty so it'll fail checkResults.
	delete(results, oldConfigFile)
	checkResults(t, expectedPaths, results)
}

func TestProcessDir_NoChanges(t *testing.T) {
	unittest.SmallTest(t)

	// Setup.
	ctx := context.Background()
	srcDir := "skia-public/templates"
	dstDir := "skia-public"
	vars := config_vars.FakeVars()
	mockRepo := &mocks.GitilesRepo{}
	mockCommit := "abc123"
	tmplBaseName := "test.tmpl"
	tmplPath := srcDir + "/" + tmplBaseName

	// Mock the interactions with Gitiles.
	configFile := "skia-public/skia-skiabot-test-autoroll.cfg"
	mockPaths := []string{strings.SplitN(configFile, "/", 2)[1]}
	mockRepo.On("ListFilesRecursiveAtRef", testutils.AnyContext, dstDir, mockCommit).Return(mockPaths, nil)
	mockRepo.On("ListFilesRecursiveAtRef", testutils.AnyContext, srcDir, mockCommit).Return([]string{tmplBaseName}, nil)
	for _, path := range mockPaths {
		// Generate the contents of the file.
		results, err := process(ctx, tmplPath, staticTemplate, "skia-public", vars)
		require.NoError(t, err)
		contents, ok := results[configFile]
		require.True(t, ok)
		mockRepo.On("ReadFileAtRef", testutils.AnyContext, strings.Join([]string{"skia-public", path}, "/"), mockCommit).Return([]byte(contents), nil)
	}
	mockRepo.On("ReadFileAtRef", testutils.AnyContext, tmplPath, mockCommit).Return([]byte(staticTemplate), nil)

	// Compute the results.
	results, err := processDir(ctx, "skia-public/templates", "skia-public", vars, mockRepo, mockCommit)
	require.NoError(t, err)
	require.Len(t, results, 0)
}
