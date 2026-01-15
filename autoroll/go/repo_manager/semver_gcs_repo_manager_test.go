package repo_manager

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/gerrit"
	gerrit_mocks "go.skia.org/infra/go/gerrit/mocks"
	"go.skia.org/infra/go/git"
	gitiles_mocks "go.skia.org/infra/go/gitiles/mocks"
	"go.skia.org/infra/go/mockhttpclient"
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
	afdoGsPrefix = "chromeos-chrome-amd64-" // Prefix of the version regex.

	// Example name: chromeos-chrome-amd64-63.0.3239.57_rc-r1.afdo.bz2
	afdoVersionRegex = ("^chromeos-chrome-amd64-" + // Prefix
		"(\\d+)\\.(\\d+)\\.(\\d+)\\.0" + // Version
		"_rc-r(\\d+)" + // Revision
		"-merged\\.afdo\\.bz2$") // Suffix
	afdoShortRevRegex = "(\\d+)\\.(\\d+)\\.(\\d+)\\.0_rc-r(\\d+)-merged"

	afdoVersionFilePath = "chrome/android/profiles/newest.txt"
)

func afdoCfg() *config.ParentChildRepoManagerConfig {
	return &config.ParentChildRepoManagerConfig{
		Parent: &config.ParentChildRepoManagerConfig_GitilesParent{
			GitilesParent: &config.GitilesParentConfig{
				Gitiles: &config.GitilesConfig{
					Branch:  git.MainBranch,
					RepoUrl: "todo.git",
				},
				Dep: &config.DependencyConfig{
					Primary: &config.VersionFileConfig{
						Id: "AFDO",
						File: []*config.VersionFileConfig_File{
							{Path: afdoVersionFilePath},
						},
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

func gerritCR(t *testing.T, g gerrit.GerritInterface, client *http.Client) codereview.CodeReview {
	rv, err := codereview.NewGerrit(&config.GerritConfig{
		Url:     "https://skia-review.googlesource.com",
		Project: "skia",
		Config:  config.GerritConfig_CHROMIUM,
	}, g, client)
	require.NoError(t, err)
	return rv
}

func setupAfdo(t *testing.T) (*parentChildRepoManager, *gitiles_mocks.GitilesRepo, *gerrit_mocks.GerritInterface, *mockhttpclient.URLMock) {
	cfg := afdoCfg()

	parentCfg := cfg.GetGitilesParent()
	p, parentGitiles, parentGerrit := parent.NewGitilesFileForTesting(t, parentCfg)

	urlmock := mockhttpclient.NewURLMock()
	c, err := child.NewSemVerGCS(t.Context(), cfg.GetSemverGcsChild(), urlmock.Client())
	require.NoError(t, err)

	// Create the RepoManager.
	rm := &parentChildRepoManager{
		Parent: p,
		Child:  c,
	}

	// Mock requests for Update().
	fileContents := map[string]string{
		afdoVersionFilePath: afdoRevBase,
	}
	parent.MockGitilesFileForUpdate(parentGitiles, cfg.GetGitilesParent(), noCheckoutParentHead, fileContents)
	mockGSList(t, urlmock, afdoGsBucket, afdoGsPath, afdoGsPrefix, map[string]string{
		afdoRevBase: afdoTimeBase,
	})
	// Mock the "list" call twice, since Update uses both LogRevisions and getAllRevisions.
	mockGSList(t, urlmock, afdoGsBucket, afdoGsPath, afdoGsPrefix, map[string]string{
		afdoRevBase: afdoTimeBase,
	})
	mockGSObject(t, urlmock, afdoGsBucket, afdoGsPath, afdoRevBase, afdoTimeBase)

	// Initial update. Everything up to date.
	_, _, _ = updateAndAssert(t, rm, parentGitiles, parentGerrit, urlmock)
	return rm, parentGitiles, parentGerrit, urlmock
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

func mockGSList(t *testing.T, urlmock *mockhttpclient.URLMock, bucket, gsPath, gsPrefix string, items map[string]string) {
	fakeUrl := fmt.Sprintf("https://storage.googleapis.com/storage/v1/b/%s/o?alt=json&delimiter=&endOffset=&includeFoldersAsPrefixes=false&includeTrailingDelimiter=false&matchGlob=&pageToken=&prefix=%s&prettyPrint=false&projection=full&startOffset=&versions=false", bucket, url.PathEscape(path.Join(gsPath, gsPrefix)))
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
	rm, parentGitiles, parentGerrit, urlmock := setupAfdo(t)
	cfg := afdoCfg()

	// Mock requests for Update.
	oldContent := map[string]string{
		afdoVersionFilePath: afdoRevBase,
	}
	parent.MockGitilesFileForUpdate(parentGitiles, cfg.GetGitilesParent(), noCheckoutParentHead, oldContent)
	mockGSList(t, urlmock, afdoGsBucket, afdoGsPath, afdoGsPrefix, map[string]string{
		afdoRevBase: afdoTimeBase,
	})
	// Mock the "list" call twice, since Update uses both LogRevisions and getAllRevisions.
	mockGSList(t, urlmock, afdoGsBucket, afdoGsPath, afdoGsPrefix, map[string]string{
		afdoRevBase: afdoTimeBase,
	})
	mockGSObject(t, urlmock, afdoGsBucket, afdoGsPath, afdoRevBase, afdoTimeBase)

	// Update.
	lastRollRev, tipRev, notRolledRevs := updateAndAssert(t, rm, parentGitiles, parentGerrit, urlmock)
	require.Equal(t, afdoRevBase, lastRollRev.Id)
	require.Equal(t, afdoRevBase, tipRev.Id)
	mockGSObject(t, urlmock, afdoGsBucket, afdoGsPath, afdoRevPrev, afdoTimePrev)
	prev, err := rm.GetRevision(t.Context(), afdoRevPrev)
	require.NoError(t, err)
	require.Equal(t, afdoRevPrev, prev.Id)
	mockGSObject(t, urlmock, afdoGsBucket, afdoGsPath, afdoRevBase, afdoTimeBase)
	base, err := rm.GetRevision(t.Context(), afdoRevBase)
	require.NoError(t, err)
	require.Equal(t, afdoRevBase, base.Id)
	mockGSObject(t, urlmock, afdoGsBucket, afdoGsPath, afdoRevNext, afdoTimeNext)
	next, err := rm.GetRevision(t.Context(), afdoRevNext)
	require.NoError(t, err)
	require.Equal(t, afdoRevNext, next.Id)
	require.Equal(t, 0, len(notRolledRevs))

	// There's a new version.
	parent.MockGitilesFileForUpdate(parentGitiles, cfg.GetGitilesParent(), noCheckoutParentHead, oldContent)
	mockGSList(t, urlmock, afdoGsBucket, afdoGsPath, afdoGsPrefix, map[string]string{
		afdoRevBase: afdoTimeBase,
		afdoRevNext: afdoTimeNext,
	})
	// Mock the "list" call twice, since Update uses both LogRevisions and getAllRevisions.
	mockGSList(t, urlmock, afdoGsBucket, afdoGsPath, afdoGsPrefix, map[string]string{
		afdoRevBase: afdoTimeBase,
		afdoRevNext: afdoTimeNext,
	})
	mockGSObject(t, urlmock, afdoGsBucket, afdoGsPath, afdoRevBase, afdoTimeBase)
	lastRollRev, tipRev, notRolledRevs = updateAndAssert(t, rm, parentGitiles, parentGerrit, urlmock)
	require.Equal(t, afdoRevBase, lastRollRev.Id)
	require.Equal(t, afdoRevNext, tipRev.Id)
	require.Equal(t, 1, len(notRolledRevs))
	require.Equal(t, afdoRevNext, notRolledRevs[0].Id)

	// Upload a CL.
	newContent := map[string]string{
		afdoVersionFilePath: afdoRevNext + "\n",
	}
	parent.MockGitilesFileForCreateNewRoll(parentGitiles, parentGerrit, cfg.GetGitilesParent(), noCheckoutParentHead, fakeCommitMsgMock, oldContent, newContent, fakeReviewers)
	_, err = rm.CreateNewRoll(t.Context(), lastRollRev, tipRev, notRolledRevs, fakeReviewers, false, false, fakeCommitMsg)
	require.NoError(t, err)
	assertExpectations(t, parentGitiles, parentGerrit, urlmock)
}

func TestAFDORepoManagerCurrentRevNotFound(t *testing.T) {
	rm, parentGitiles, parentGerrit, urlmock := setupAfdo(t)
	cfg := afdoCfg()

	// Sanity check.
	mockGSObject(t, urlmock, afdoGsBucket, afdoGsPath, afdoRevPrev, afdoTimePrev)
	prev, err := rm.GetRevision(t.Context(), afdoRevPrev)
	require.NoError(t, err)
	require.Equal(t, afdoRevPrev, prev.Id)
	mockGSObject(t, urlmock, afdoGsBucket, afdoGsPath, afdoRevBase, afdoTimeBase)
	base, err := rm.GetRevision(t.Context(), afdoRevBase)
	require.NoError(t, err)
	require.Equal(t, afdoRevBase, base.Id)
	mockGSObject(t, urlmock, afdoGsBucket, afdoGsPath, afdoRevNext, afdoTimeNext)
	next, err := rm.GetRevision(t.Context(), afdoRevNext)
	require.NoError(t, err)
	require.Equal(t, afdoRevNext, next.Id)

	// We've rolled to a revision which is not in the GCS bucket.
	fileContents := map[string]string{
		afdoVersionFilePath: "BOGUS_REV",
	}
	parent.MockGitilesFileForUpdate(parentGitiles, cfg.GetGitilesParent(), noCheckoutParentHead, fileContents)
	mockGSList(t, urlmock, afdoGsBucket, afdoGsPath, afdoGsPrefix, map[string]string{
		afdoRevBase: afdoTimeBase,
		afdoRevPrev: afdoTimePrev,
		afdoRevNext: afdoTimeNext,
	})
	mockGSObject(t, urlmock, afdoGsBucket, afdoGsPath, "BOGUS_REV", afdoTimePrev)
	lastRollRev, tipRev, notRolledRevs := updateAndAssert(t, rm, parentGitiles, parentGerrit, urlmock)
	expect := &revision.Revision{
		Id:       "BOGUS_REV",
		Checksum: "76c69f92576495db1a",
		Display:  "BOGUS_REV",
		URL:      "https://storage.googleapis.com/storage/v1/b/chromeos-prebuilt/o/afdo-job%2Fllvm%2FBOGUS_REV?alt=json&prettyPrint=false&projection=full",
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
	parent.MockGitilesFileForUpdate(parentGitiles, cfg.GetGitilesParent(), noCheckoutParentHead, fileContents)
	mockGSList(t, urlmock, afdoGsBucket, afdoGsPath, afdoGsPrefix, map[string]string{
		afdoRevBase: afdoTimeBase,
		afdoRevPrev: afdoTimePrev,
		afdoRevNext: afdoTimeNext,
	})
	// Mock the "list" call twice, since GetRevision falls back to scanning the
	// GCS directory after the initial GetFileObjectAttrs call fails.
	mockGSList(t, urlmock, afdoGsBucket, afdoGsPath, "", map[string]string{
		afdoRevBase: afdoTimeBase,
		afdoRevPrev: afdoTimePrev,
		afdoRevNext: afdoTimeNext,
	})
	lastRollRev, tipRev, notRolledRevs = updateAndAssert(t, rm, parentGitiles, parentGerrit, urlmock)
	assertdeep.Equal(t, &revision.Revision{
		Id:            "BOGUS_REV",
		InvalidReason: "Failed to retrieve revision.",
	}, lastRollRev)
	require.Equal(t, afdoRevNext, tipRev.Id)
	require.Equal(t, 1, len(notRolledRevs))
	require.Equal(t, afdoRevNext, notRolledRevs[0].Id)
	require.True(t, urlmock.Empty())
}
