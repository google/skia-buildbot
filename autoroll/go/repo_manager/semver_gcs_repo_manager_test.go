package repo_manager

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
	gitiles_testutils "go.skia.org/infra/go/gitiles/testutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
)

const (
	afdoRevPrev = "chromeos-chrome-amd64-66.0.3336.0_rc-r0-merged.afdo.bz2"
	afdoRevBase = "chromeos-chrome-amd64-66.0.3336.0_rc-r1-merged.afdo.bz2"
	afdoRevNext = "chromeos-chrome-amd64-66.0.3337.0_rc-r1-merged.afdo.bz2"

	afdoTimePrev = "2009-11-10T23:00:00Z"
	afdoTimeBase = "2009-11-10T23:01:00Z"
	afdoTimeNext = "2009-11-10T23:02:00Z"

	afdoGsBucket = "chromeos-prebuilt"
	afdoGsPath   = "afdo-job/llvm"

	// Example name: chromeos-chrome-amd64-63.0.3239.57_rc-r1.afdo.bz2
	afdoVersionRegex = ("^chromeos-chrome-amd64-" + // Prefix
		"(\\d+)\\.(\\d+)\\.(\\d+)\\.0" + // Version
		"_rc-r(\\d+)" + // Revision
		"-merged\\.afdo\\.bz2$") // Suffix
	afdoShortRevRegex = "(\\d+)\\.(\\d+)\\.(\\d+)\\.0_rc-r(\\d+)-merged"

	afdoVersionFilePath = "chrome/android/profiles/newest.txt"
)

func afdoCfg(t *testing.T) *config.ParentChildRepoManagerConfig {
	return &config.ParentChildRepoManagerConfig{
		Parent: &config.ParentChildRepoManagerConfig_GitilesParent{
			GitilesParent: &config.GitilesParentConfig{
				Gitiles: &config.GitilesConfig{
					Branch:  git.DefaultBranch,
					RepoUrl: "todo.git",
				},
				Dep: &config.DependencyConfig{
					Primary: &config.VersionFileConfig{
						Id:   "AFDO",
						Path: afdoVersionFilePath,
					},
				},
				Gerrit: &config.GerritConfig{
					Url:     "https://fake-skia-review.googlesource.com",
					Project: "fake-gerrit-project",
					Config:  config.GerritConfig_CHROMIUM,
				},
			},
		},
		Child: &config.ParentChildRepoManagerConfig_SemverGcsChild{
			SemverGcsChild: &config.SemVerGCSChildConfig{
				Gcs: &config.GCSChildConfig{
					GcsBucket: afdoGsBucket,
					GcsPath:   afdoGsPath,
				},
				ShortRevRegex: afdoShortRevRegex,
				VersionRegex:  afdoVersionRegex,
			},
		},
	}
}

func gerritCR(t *testing.T, g gerrit.GerritInterface) codereview.CodeReview {
	rv, err := codereview.NewGerrit(&config.GerritConfig{
		Url:     "https://skia-review.googlesource.com",
		Project: "skia",
		Config:  config.GerritConfig_CHROMIUM,
	}, g)
	require.NoError(t, err)
	return rv
}

func setupAfdo(t *testing.T) (context.Context, *parentChildRepoManager, *mockhttpclient.URLMock, *gitiles_testutils.MockRepo, *git_testutils.GitBuilder, func()) {
	wd, err := ioutil.TempDir("", "")
	require.NoError(t, err)

	ctx := context.Background()

	// Create child and parent repos.
	parent := git_testutils.GitInit(t, ctx)
	parent.Add(context.Background(), afdoVersionFilePath, afdoRevBase)
	parent.Commit(context.Background())

	urlmock := mockhttpclient.NewURLMock()
	mockParent := gitiles_testutils.NewMockRepo(t, parent.RepoUrl(), git.GitDir(parent.Dir()), urlmock)

	gUrl := "https://fake-skia-review.googlesource.com"
	serialized, err := json.Marshal(&gerrit.AccountDetails{
		AccountId: 101,
		Name:      mockUser,
		Email:     mockUser,
		UserName:  mockUser,
	})
	require.NoError(t, err)
	serialized = append([]byte("abcd\n"), serialized...)
	urlmock.MockOnce(gUrl+"/a/accounts/self/detail", mockhttpclient.MockGetDialogue(serialized))
	g, err := gerrit.NewGerrit(gUrl, urlmock.Client())
	require.NoError(t, err)

	cfg := afdoCfg(t)
	parentCfg := cfg.Parent.(*config.ParentChildRepoManagerConfig_GitilesParent).GitilesParent
	parentCfg.Gitiles.RepoUrl = parent.RepoUrl()

	rm, err := newParentChildRepoManager(ctx, cfg, setupRegistry(t), wd, "fake-roller", "fake-recipe-cfg", "fake.server.com", urlmock.Client(), gerritCR(t, g))
	require.NoError(t, err)

	// Mock requests for Update.
	mockParent.MockGetCommit(ctx, git.DefaultBranch)
	parentHead, err := git.GitDir(parent.Dir()).RevParse(ctx, "HEAD")
	require.NoError(t, err)
	mockParent.MockReadFile(ctx, afdoVersionFilePath, parentHead)
	mockGSList(t, urlmock, afdoGsBucket, afdoGsPath, map[string]string{
		afdoRevBase: afdoTimeBase,
	})
	mockGSObject(t, urlmock, afdoGsBucket, afdoGsPath, afdoRevBase, afdoTimeBase)

	// Initial update. Everything up to date.
	_, _, _, err = rm.Update(ctx)
	require.NoError(t, err)

	cleanup := func() {
		testutils.RemoveAll(t, wd)
		parent.Cleanup()
	}

	return ctx, rm, urlmock, mockParent, parent, cleanup
}

type gsObject struct {
	Kind                    string `json:"kind"`
	Id                      string `json:"id"`
	SelfLink                string `json:"selfLink"`
	Name                    string `json:"name"`
	Bucket                  string `json:"bucket"`
	Generation              string `json:"generation"`
	Metageneration          string `json:"metageneration"`
	ContentType             string `json:"contentType"`
	TimeCreated             string `json:"timeCreated"`
	Updated                 string `json:"updated"`
	StorageClass            string `json:"storageClass"`
	TimeStorageClassUpdated string `json:"timeStorageClassUpdated"`
	Size                    string `json:"size"`
	Md5Hash                 string `json:"md5Hash"`
	MediaLink               string `json:"mediaLink"`
	Crc32c                  string `json:"crc32c"`
	Etag                    string `json:"etag"`
}

type gsObjectList struct {
	Kind  string     `json:"kind"`
	Items []gsObject `json:"items"`
}

func mockGSList(t *testing.T, urlmock *mockhttpclient.URLMock, bucket, gsPath string, items map[string]string) {
	fakeUrl := fmt.Sprintf("https://storage.googleapis.com/storage/v1/b/%s/o?alt=json&delimiter=&endOffset=&pageToken=&prefix=%s&prettyPrint=false&projection=full&startOffset=&versions=false", bucket, url.PathEscape(gsPath))
	resp := gsObjectList{
		Kind:  "storage#objects",
		Items: []gsObject{},
	}
	for item, timestamp := range items {
		resp.Items = append(resp.Items, gsObject{
			Kind:                    "storage#object",
			Id:                      path.Join(bucket+gsPath, item),
			SelfLink:                path.Join(bucket+gsPath, item),
			Name:                    item,
			Bucket:                  bucket,
			Generation:              "1",
			Metageneration:          "1",
			ContentType:             "application/octet-stream",
			TimeCreated:             timestamp,
			Updated:                 timestamp,
			StorageClass:            "MULTI_REGIONAL",
			TimeStorageClassUpdated: timestamp,
			Size:                    "12345",
			Md5Hash:                 "dsafkldkldsaf",
			MediaLink:               fakeUrl + item,
			Crc32c:                  "eiekls",
			Etag:                    "lasdfklds",
		})
	}
	respBytes, err := json.MarshalIndent(resp, "", "  ")
	require.NoError(t, err)
	urlmock.MockOnce(fakeUrl, mockhttpclient.MockGetDialogue(respBytes))
}

func mockGSObject(t *testing.T, urlmock *mockhttpclient.URLMock, bucket, gsPath, item, timestamp string) {
	fakeUrl := fmt.Sprintf("https://storage.googleapis.com/storage/v1/b/%s/o/%s?alt=json&prettyPrint=false&projection=full", bucket, url.PathEscape(path.Join(gsPath, item)))
	resp := gsObject{
		Kind:                    "storage#object",
		Id:                      path.Join(bucket+gsPath, item),
		SelfLink:                path.Join(bucket+gsPath, item),
		Name:                    item,
		Bucket:                  bucket,
		Generation:              "1",
		Metageneration:          "1",
		ContentType:             "application/octet-stream",
		TimeCreated:             timestamp,
		Updated:                 timestamp,
		StorageClass:            "MULTI_REGIONAL",
		TimeStorageClassUpdated: timestamp,
		Size:                    "12345",
		Md5Hash:                 "dsafkldkldsaf",
		MediaLink:               fakeUrl,
		Crc32c:                  "eiekls",
		Etag:                    "lasdfklds",
	}
	respBytes, err := json.MarshalIndent(resp, "", "  ")
	require.NoError(t, err)
	urlmock.MockOnce(fakeUrl, mockhttpclient.MockGetDialogue(respBytes))
}

func TestAFDORepoManager(t *testing.T) {
	unittest.LargeTest(t)

	ctx, rm, urlmock, mockParent, parent, cleanup := setupAfdo(t)
	defer cleanup()

	// Mock requests for Update.
	mockParent.MockGetCommit(ctx, git.DefaultBranch)
	parentHead, err := git.GitDir(parent.Dir()).RevParse(ctx, "HEAD")
	require.NoError(t, err)
	mockParent.MockReadFile(ctx, afdoVersionFilePath, parentHead)
	mockGSList(t, urlmock, afdoGsBucket, afdoGsPath, map[string]string{
		afdoRevBase: afdoTimeBase,
	})
	mockGSObject(t, urlmock, afdoGsBucket, afdoGsPath, afdoRevBase, afdoTimeBase)

	// Update.
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)
	require.Equal(t, afdoRevBase, lastRollRev.Id)
	require.Equal(t, afdoRevBase, tipRev.Id)
	mockGSObject(t, urlmock, afdoGsBucket, afdoGsPath, afdoRevPrev, afdoTimePrev)
	prev, err := rm.GetRevision(ctx, afdoRevPrev)
	require.NoError(t, err)
	require.Equal(t, afdoRevPrev, prev.Id)
	mockGSObject(t, urlmock, afdoGsBucket, afdoGsPath, afdoRevBase, afdoTimeBase)
	base, err := rm.GetRevision(ctx, afdoRevBase)
	require.NoError(t, err)
	require.Equal(t, afdoRevBase, base.Id)
	mockGSObject(t, urlmock, afdoGsBucket, afdoGsPath, afdoRevNext, afdoTimeNext)
	next, err := rm.GetRevision(ctx, afdoRevNext)
	require.NoError(t, err)
	require.Equal(t, afdoRevNext, next.Id)
	require.Equal(t, 0, len(notRolledRevs))

	// There's a new version.
	mockParent.MockGetCommit(ctx, git.DefaultBranch)
	mockParent.MockReadFile(ctx, afdoVersionFilePath, parentHead)
	mockGSList(t, urlmock, afdoGsBucket, afdoGsPath, map[string]string{
		afdoRevBase: afdoTimeBase,
		afdoRevNext: afdoTimeNext,
	})
	mockGSObject(t, urlmock, afdoGsBucket, afdoGsPath, afdoRevBase, afdoTimeBase)
	lastRollRev, tipRev, notRolledRevs, err = rm.Update(ctx)
	require.NoError(t, err)
	require.Equal(t, afdoRevBase, lastRollRev.Id)
	require.Equal(t, afdoRevNext, tipRev.Id)
	require.Equal(t, 1, len(notRolledRevs))
	require.Equal(t, afdoRevNext, notRolledRevs[0].Id)

	// Upload a CL.

	// Mock the request to get the current version.
	mockParent.MockReadFile(ctx, afdoVersionFilePath, parentHead)

	// Mock the initial change creation.
	subject := strings.Split(fakeCommitMsg, "\n")[0]
	reqBody := []byte(fmt.Sprintf(`{"project":"%s","subject":"%s","branch":"%s","topic":"","status":"NEW","base_commit":"%s"}`, "fake-gerrit-project", subject, git.DefaultBranch, parentHead))
	ci := gerrit.ChangeInfo{
		ChangeId: "123",
		Id:       "123",
		Issue:    123,
		Revisions: map[string]*gerrit.Revision{
			"ps1": {
				ID:     "ps1",
				Number: 1,
			},
		},
	}
	respBody, err := json.Marshal(ci)
	require.NoError(t, err)
	respBody = append([]byte(")]}'\n"), respBody...)
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/", mockhttpclient.MockPostDialogueWithResponseCode("application/json", reqBody, respBody, 201))

	// Mock the edit of the change to update the commit message.
	reqBody = []byte(fmt.Sprintf(`{"message":"%s"}`, strings.Replace(fakeCommitMsg, "\n", "\\n", -1)))
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/edit:message", mockhttpclient.MockPutDialogue("application/json", reqBody, []byte("")))

	// Mock the request to modify the version file.
	reqBody = []byte(tipRev.Id + "\n")
	url := fmt.Sprintf("https://fake-skia-review.googlesource.com/a/changes/123/edit/%s", url.QueryEscape(afdoVersionFilePath))
	urlmock.MockOnce(url, mockhttpclient.MockPutDialogue("", reqBody, []byte("")))

	// Mock the request to publish the change edit.
	reqBody = []byte(`{"notify":"ALL"}`)
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/edit:publish", mockhttpclient.MockPostDialogue("application/json", reqBody, []byte("")))

	// Mock the request to load the updated change.
	respBody, err = json.Marshal(ci)
	require.NoError(t, err)
	respBody = append([]byte(")]}'\n"), respBody...)
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/detail?o=ALL_REVISIONS", mockhttpclient.MockGetDialogue(respBody))

	// Mock the request to set the CQ.
	reqBody = []byte(`{"labels":{"Code-Review":1,"Commit-Queue":2},"message":"","reviewers":[{"reviewer":"reviewer@chromium.org"}]}`)
	urlmock.MockOnce("https://fake-skia-review.googlesource.com/a/changes/123/revisions/ps1/review", mockhttpclient.MockPostDialogue("application/json", reqBody, []byte("")))

	issue, err := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, emails, false, fakeCommitMsg)
	require.NoError(t, err)
	require.Equal(t, ci.Issue, issue)
}

func TestAFDORepoManagerCurrentRevNotFound(t *testing.T) {
	unittest.LargeTest(t)

	ctx, rm, urlmock, mockParent, parent, cleanup := setupAfdo(t)
	defer cleanup()

	// Sanity check.
	mockGSObject(t, urlmock, afdoGsBucket, afdoGsPath, afdoRevPrev, afdoTimePrev)
	prev, err := rm.GetRevision(ctx, afdoRevPrev)
	require.NoError(t, err)
	require.Equal(t, afdoRevPrev, prev.Id)
	mockGSObject(t, urlmock, afdoGsBucket, afdoGsPath, afdoRevBase, afdoTimeBase)
	base, err := rm.GetRevision(ctx, afdoRevBase)
	require.NoError(t, err)
	require.Equal(t, afdoRevBase, base.Id)
	mockGSObject(t, urlmock, afdoGsBucket, afdoGsPath, afdoRevNext, afdoTimeNext)
	next, err := rm.GetRevision(ctx, afdoRevNext)
	require.NoError(t, err)
	require.Equal(t, afdoRevNext, next.Id)

	// Roll to a revision which is not in the GCS bucket.
	parent.Add(context.Background(), afdoVersionFilePath, "BOGUS_REV")
	parent.Commit(context.Background())
	mockParent.MockGetCommit(ctx, git.DefaultBranch)
	parentHead, err := git.GitDir(parent.Dir()).RevParse(ctx, "HEAD")
	require.NoError(t, err)
	mockParent.MockReadFile(ctx, afdoVersionFilePath, parentHead)
	mockGSList(t, urlmock, afdoGsBucket, afdoGsPath, map[string]string{
		afdoRevBase: afdoTimeBase,
		afdoRevPrev: afdoTimePrev,
		afdoRevNext: afdoTimeNext,
	})
	mockGSObject(t, urlmock, afdoGsBucket, afdoGsPath, "BOGUS_REV", afdoTimePrev)
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)
	expect := &revision.Revision{
		Id:      "BOGUS_REV",
		Display: "BOGUS_REV",
		URL:     "https://storage.googleapis.com/storage/v1/b/chromeos-prebuilt/o/afdo-job%2Fllvm%2FBOGUS_REV?alt=json&prettyPrint=false&projection=full",
	}
	expect.Timestamp = lastRollRev.Timestamp
	assertdeep.Equal(t, expect, lastRollRev)
	require.False(t, util.TimeIsZero(lastRollRev.Timestamp))
	require.Equal(t, afdoRevNext, tipRev.Id)
	require.Equal(t, 1, len(notRolledRevs))
	require.Equal(t, afdoRevNext, notRolledRevs[0].Id)
	require.True(t, urlmock.Empty())

	// Now try again, but don't mock the bogus rev in GCS. We should still
	// come up with the same lastRollRev.Id, but the Revision will otherwise
	// be empty.
	mockParent.MockGetCommit(ctx, git.DefaultBranch)
	mockParent.MockReadFile(ctx, afdoVersionFilePath, parentHead)
	mockGSList(t, urlmock, afdoGsBucket, afdoGsPath, map[string]string{
		afdoRevBase: afdoTimeBase,
		afdoRevPrev: afdoTimePrev,
		afdoRevNext: afdoTimeNext,
	})
	lastRollRev, tipRev, notRolledRevs, err = rm.Update(ctx)
	require.NoError(t, err)
	assertdeep.Equal(t, &revision.Revision{
		Id: "BOGUS_REV",
	}, lastRollRev)
	require.Equal(t, afdoRevNext, tipRev.Id)
	require.Equal(t, 1, len(notRolledRevs))
	require.Equal(t, afdoRevNext, notRolledRevs[0].Id)
	require.True(t, urlmock.Empty())
}
