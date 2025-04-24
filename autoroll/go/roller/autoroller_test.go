package roller

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/manual"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/autoroll/go/roller_cleanup"
	roller_cleanup_mocks "go.skia.org/infra/autoroll/go/roller_cleanup/mocks"
	"go.skia.org/infra/autoroll/go/status"
	"go.skia.org/infra/email/go/emailclient"
	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/gerrit"
	gerrit_mocks "go.skia.org/infra/go/gerrit/mocks"
	gerrit_testutils "go.skia.org/infra/go/gerrit/testutils"
	"go.skia.org/infra/go/git/git_common"
	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/testutils"
)

func TestAutoRollerRolledPast(t *testing.T) {

	ctx := context.Background()
	r := &AutoRoller{}
	rev := func(id string) *revision.Revision {
		return &revision.Revision{Id: id}
	}
	r.lastRollRev = rev("0")
	r.nextRollRev = rev("1") // Pretend we're configured to roll one rev at a time.
	r.tipRev = rev("5")
	r.notRolledRevs = []*revision.Revision{
		rev("5"),
		rev("4"),
		rev("3"),
		rev("2"),
		rev("1"),
	}

	check := func(id string, expect bool) {
		got, err := r.RolledPast(ctx, &revision.Revision{Id: id})
		require.NoError(t, err)
		require.Equal(t, expect, got)
	}

	check("0", true)              // lastRollRev
	check("1", false)             // nextRollRev
	check("2", false)             // notRolledRev
	check("3", false)             // notRolledRev
	check("4", false)             // notRolledRev
	check("5", false)             // tipRev
	check("some other rev", true) // everything else
}

func TestDeleteCheckoutAndExit(t *testing.T) {
	// Create some files and directories to be deleted. Include both normal and
	// hidden files and dirs, with nested files.
	tmp := t.TempDir()
	dirs := []string{
		filepath.Join(tmp, ".hiddendir"),
		filepath.Join(tmp, "normaldir"),
	}
	files := []string{
		filepath.Join(tmp, ".hiddenfile"),
		filepath.Join(tmp, "normalfile"),
		filepath.Join(tmp, ".hiddendir", "nested"),
		filepath.Join(tmp, "normaldir", "nested"),
	}
	for _, dir := range dirs {
		require.NoError(t, os.MkdirAll(dir, os.ModePerm))
	}
	for _, file := range files {
		require.NoError(t, os.WriteFile(file, []byte("blahblah"), os.ModePerm))
	}

	// Create the roller.
	mockCleanup := &roller_cleanup_mocks.DB{}
	r := &AutoRoller{
		cleanup: mockCleanup,
		roller:  "my-roller",
		workdir: tmp,
	}
	ts := time.Unix(1715005596, 0) // Arbitrary timestamp.
	nowProvider := func() time.Time {
		return ts
	}
	ctx := context.WithValue(context.Background(), now.ContextKey, now.NowProvider(nowProvider))

	// Mock the request to clear the needs-cleanup bit.
	mockCleanup.On("RequestCleanup", ctx, &roller_cleanup.CleanupRequest{
		RollerID:      r.roller,
		NeedsCleanup:  false,
		User:          r.roller,
		Timestamp:     ts,
		Justification: "Deleted local data",
	}).Return(nil)

	// DeleteLocalData.
	require.NoError(t, r.DeleteLocalData(ctx))

	// Ensure that tmp still exists (for most rollers this is a mounted
	// directory which we cannot delete) but is empty.
	st, err := os.Stat(tmp)
	require.NoError(t, err)
	require.True(t, st.IsDir())

	// Use os.Stat for each of the listed files and directories rather than
	// os.ReadDir, just in case that doesn't return the hidden files and dirs.
	for _, dir := range dirs {
		_, err := os.Stat(dir)
		require.True(t, os.IsNotExist(err))
	}
	for _, file := range files {
		_, err := os.Stat(file)
		require.True(t, os.IsNotExist(err))
	}
}

func TestRepoManagerInitFailed(t *testing.T) {
	// Ensure that we delete local data when repo manager creation fails.
	gerritConfig := &config.GerritConfig{
		Url:     "fake.gerrit.url",
		Project: "fake-gerrit-project",
	}
	cfg := &config.Config{
		RollerName:        "fake-roller",
		ChildDisplayName:  "child",
		ParentDisplayName: "parent",
		ParentWaterfall:   "fake",
		OwnerPrimary:      "me",
		OwnerSecondary:    "you",
		Contacts:          []string{"me@google.com"},
		ServiceAccount:    "fake-service-account@",
		Reviewer:          []string{"me@google.com"},
		CommitMsg:         &config.CommitMsgConfig{},
		CodeReview: &config.Config_Gerrit{
			Gerrit: gerritConfig,
		},
		Kubernetes: &config.KubernetesConfig{
			Cpu:    "1",
			Disk:   "128",
			Memory: "1",
			Image:  "fake-docker-image",
		},
		RepoManager: &config.Config_ParentChildRepoManager{
			ParentChildRepoManager: &config.ParentChildRepoManagerConfig{
				Parent: &config.ParentChildRepoManagerConfig_DepsLocalGerritParent{
					DepsLocalGerritParent: &config.DEPSLocalGerritParentConfig{
						DepsLocal: &config.DEPSLocalParentConfig{
							GitCheckout: &config.GitCheckoutParentConfig{
								GitCheckout: &config.GitCheckoutConfig{
									Branch:  "main",
									RepoUrl: "fake.parent.url",
								},
								Dep: &config.DependencyConfig{
									Primary: &config.VersionFileConfig{
										Id: "fake.child.url",
										File: []*config.VersionFileConfig_File{
											{
												Path: "DEPS",
											},
										},
									},
								},
							},
						},
						Gerrit: gerritConfig,
					},
				},
				Child: &config.ParentChildRepoManagerConfig_GitCheckoutChild{
					GitCheckoutChild: &config.GitCheckoutChildConfig{
						GitCheckout: &config.GitCheckoutConfig{
							Branch:  "main",
							RepoUrl: "fake.child.url",
						},
					},
				},
			},
		},
	}
	urlmock := mockhttpclient.NewURLMock()
	httpClient := urlmock.Client()
	chatbotCfgReader := func() string { return "" }
	emailer := emailclient.Client{}
	gerritClient := &gerrit_mocks.GerritInterface{}
	githubClient := (*github.GitHub)(nil)
	workdir := t.TempDir()
	serverURL := ""
	gcsClient := gcs.GCSClient(nil)
	rollerName := cfg.RollerName
	local := true
	statusDB := status.DB(nil)
	manualRollDB := manual.DB(nil)
	cleanupDB := &roller_cleanup_mocks.DB{}

	// Set up mocks.
	nowProvider := func() time.Time {
		return time.Unix(1729524544, 0) // Arbitrary timestamp.
	}
	gitPath := "/path/to/fake/git"
	ctx := context.Background()
	ctx = git_common.WithGitFinder(ctx, func() (string, error) {
		return gitPath, nil
	})
	ctx = exec.NewContext(ctx, func(ctx context.Context, cmd *exec.Command) error {
		// Mocks needed for git_common.FindGit.
		if err := git_common.MocksForFindGit(ctx, cmd); err != nil {
			return err
		}

		// Fail all gclient commands. This ensures that RepoManager creation
		// will fail, as required by this test.
		isGclientCmd := false
		for _, arg := range append([]string{cmd.Name}, cmd.Args...) {
			if strings.Contains(arg, "gclient") {
				isGclientCmd = true
			}
		}
		if isGclientCmd {
			return errors.New("mocked gclient error")
		}

		// Misc Git mocks.
		sklog.Errorf("%s %v", cmd.Name, cmd.Args)
		if cmd.Name == gitPath {
			// This is for syncing depot tools. Just return the expected hash.
			if cmd.Args[0] == "rev-parse" {
				depotToolsVersion, err := depot_tools.FindVersion()
				if err != nil {
					return err
				}
				_, err = cmd.CombinedOutput.Write([]byte(depotToolsVersion))
				return err
			}
		}

		return nil
	})
	ctx = context.WithValue(ctx, now.ContextKey, now.NowProvider(nowProvider))

	gerritClient.On("GetUserEmail", testutils.AnyContext).Return("me@google.com", nil)
	urlmock.Mock("https://chromiumdash.appspot.com/fetch_milestones", mockhttpclient.MockGetDialogue([]byte(`[
{"chromium_branch":"6778","milestone":131,"schedule_active":true,"schedule_phase":"beta"},
{"chromium_branch":"6723","milestone":130,"schedule_active":true,"schedule_phase":"stable"}
]`)))
	cleanupDB.On("RequestCleanup", testutils.AnyContext, &roller_cleanup.CleanupRequest{
		RollerID:      "fake-roller",
		NeedsCleanup:  false,
		User:          "fake-roller",
		Timestamp:     now.Now(ctx),
		Justification: "Deleted local data",
	}).Return(nil)

	// Attempt to create the roller, ensure that it fails.
	_, err := NewAutoRoller(ctx, cfg, emailer, chatbotCfgReader, gerritClient, githubClient, workdir, serverURL, gcsClient, httpClient, rollerName, local, statusDB, manualRollDB, cleanupDB)
	require.ErrorContains(t, err, "mocked gclient error")

	// Ensure all of our mocks were called.
	gerritClient.AssertExpectations(t)
	require.True(t, urlmock.Empty())
	cleanupDB.AssertExpectations(t)
}

func TestIsRevisionNotSubmitted(t *testing.T) {
	test := func(name, expect string, req *manual.ManualRollRequest, rev *revision.Revision, ci *gerrit.ChangeInfo) {
		t.Run(name, func(t *testing.T) {
			mockGerrit := gerrit_testutils.NewGerrit(t)
			if ci != nil {
				mockGerrit.MockGetIssueProperties(ci)
			}
			actual, err := isRevisionNotSubmitted(context.Background(), req, rev, mockGerrit.Mock.Client())
			require.NoError(t, err)
			require.Equal(t, expect, actual)
		})
	}

	test("NoResolveRevision", "NoResolveRevision is set", &manual.ManualRollRequest{
		NoResolveRevision: true,
	}, nil, nil)

	test("Canary", "Canary is set", &manual.ManualRollRequest{
		Canary: true,
	}, nil, nil)

	test("Not a Git revision", "", &manual.ManualRollRequest{}, &revision.Revision{}, nil)

	test("Not a Gerrit change", "Revision is not a Gerrit change; cannot verify that it has been reviewed and submitted", &manual.ManualRollRequest{}, &revision.Revision{
		Id:      "fake-commit",
		IsGit:   true,
		Details: "some commit message with no footers",
	}, nil)

	test("Not merged", fmt.Sprintf("CL %s/c/12345 is not merged", gerrit_testutils.FakeGerritURL), &manual.ManualRollRequest{}, &revision.Revision{
		Id:    "fake-commit",
		IsGit: true,
		Details: `some commit message

Change-Id: 12345
`,
		URL: strings.Replace(gerrit_testutils.FakeGerritURL, "-review", "", 1),
	}, &gerrit.ChangeInfo{
		Issue:    12345,
		ChangeId: "12345",
	})

	test("Merged", "", &manual.ManualRollRequest{}, &revision.Revision{
		Id:    "fake-commit",
		IsGit: true,
		Details: `some commit message

Change-Id: 56789
`,
		URL: strings.Replace(gerrit_testutils.FakeGerritURL, "-review", "", 1),
	}, &gerrit.ChangeInfo{
		Issue:    56789,
		ChangeId: "56789",
		Status:   gerrit.ChangeStatusMerged,
	})
}
