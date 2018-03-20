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

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/autoroll"
	depot_tools "go.skia.org/infra/go/depot_tools/testutils"
	"go.skia.org/infra/go/exec"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
)

const (
	afdoRevPrev = "chromeos-chrome-amd64-66.0.3336.3_rc-r0.afdo.bz2"
	afdoRevBase = "chromeos-chrome-amd64-66.0.3336.3_rc-r1.afdo.bz2"
	afdoRevNext = "chromeos-chrome-amd64-66.0.3337.3_rc-r1.afdo.bz2"

	afdoTimePrev = "2009-11-10T23:00:00Z"
	afdoTimeBase = "2009-11-10T23:01:00Z"
	afdoTimeNext = "2009-11-10T23:02:00Z"
)

func setupAfdo(t *testing.T) (context.Context, string, *git_testutils.GitBuilder, *exec.CommandCollector, *mockhttpclient.URLMock, func()) {
	wd, err := ioutil.TempDir("", "")
	assert.NoError(t, err)

	// Create child and parent repos.
	parent := git_testutils.GitInit(t, context.Background())
	parent.Add(context.Background(), AFDO_VERSION_FILE_PATH, afdoRevBase)
	parent.Commit(context.Background())

	mockRun := &exec.CommandCollector{}
	mockRun.SetDelegateRun(func(cmd *exec.Command) error {
		if cmd.Name == "git" && cmd.Args[0] == "cl" {
			if cmd.Args[1] == "upload" {
				return nil
			} else if cmd.Args[1] == "issue" {
				json := testutils.MarshalJSON(t, &issueJson{
					Issue:    issueNum,
					IssueUrl: "???",
				})
				f := strings.Split(cmd.Args[2], "=")[1]
				testutils.WriteFile(t, f, json)
				return nil
			}
		}
		return exec.DefaultRun(cmd)
	})
	ctx := exec.NewContext(context.Background(), mockRun.Run)
	urlmock := mockhttpclient.NewURLMock()

	cleanup := func() {
		testutils.RemoveAll(t, wd)
		parent.Cleanup()
	}

	return ctx, wd, parent, mockRun, urlmock, cleanup
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

func TestParseAFDOVersion(t *testing.T) {
	testutils.SmallTest(t)

	// Success cases.
	testS := func(s string, expect [AFDO_VERSION_LENGTH]int) {
		actual, err := parseAFDOVersion(s)
		assert.NoError(t, err)
		assert.Equal(t, expect, actual)
	}
	testS(afdoRevPrev, [AFDO_VERSION_LENGTH]int{66, 0, 3336, 3, 0})
	testS(afdoRevBase, [AFDO_VERSION_LENGTH]int{66, 0, 3336, 3, 1})
	testS(afdoRevNext, [AFDO_VERSION_LENGTH]int{66, 0, 3337, 3, 1})
	testS("chromeos-chrome-amd64-67.0.3.222222_rc-r32823.afdo.bz2", [AFDO_VERSION_LENGTH]int{67, 0, 3, 222222, 32823})

	// Failure cases.
	testF := func(s string) {
		_, err := parseAFDOVersion(s)
		assert.NotNil(t, err)
	}
	testF("chromeos-chrome-amd64-66.0.3336.3_rc-rr.afdo.bz2")
	testF("chromeos-chrome-amd64-66.0.3336.d_rc-r1.afdo.bz2")
	testF("chromeos-chrome-amd64-66.0.333b.3_rc-r1.afdo.bz2")
	testF("chromeos-chrome-amd64-L6.0.3336.3_rc-r1.afdo.bz2")
	testF("66.0.3336.3_rc-r1")
	testF("chromeos-chrome-amd64-66.0.3336.3_rc-r1")
	testF("66.0.3336.3_rc-rr.afdo.bz2")
	testF("")
}

func TestAFDOVersionGreater(t *testing.T) {
	testutils.SmallTest(t)

	// Success cases.
	test := func(a, b string, expect bool) {
		actual, err := afdoVersionGreater(a, b)
		assert.NoError(t, err)
		assert.Equal(t, expect, actual)
	}
	test(afdoRevPrev, afdoRevBase, false)
	test(afdoRevBase, afdoRevPrev, true)
	test(afdoRevBase, afdoRevBase, false)
	test(afdoRevBase, afdoRevNext, false)
	test(afdoRevNext, afdoRevBase, true)
	test(afdoRevPrev, afdoRevNext, false)
	test(afdoRevNext, afdoRevPrev, true)

	t2 := func(a, b [AFDO_VERSION_LENGTH]int, expect bool) {
		tmpl := "chromeos-chrome-amd64-%d.%d.%d.%d_rc-r%d.afdo.bz2"
		verA := fmt.Sprintf(tmpl, a[0], a[1], a[2], a[3], a[4])
		verB := fmt.Sprintf(tmpl, b[0], b[1], b[2], b[3], b[4])
		test(verA, verB, expect)
	}

	t2([AFDO_VERSION_LENGTH]int{66, 0, 3336, 3, 1}, [AFDO_VERSION_LENGTH]int{64, 0, 3282, 165, 1}, true)
	t2([AFDO_VERSION_LENGTH]int{64, 0, 3282, 165, 1}, [AFDO_VERSION_LENGTH]int{66, 0, 3336, 3, 1}, false)
	t2([AFDO_VERSION_LENGTH]int{5, 5, 5, 5, 5}, [AFDO_VERSION_LENGTH]int{5, 5, 5, 5, 5}, false)
	t2([AFDO_VERSION_LENGTH]int{5, 5, 5, 5, 5}, [AFDO_VERSION_LENGTH]int{5, 5, 5, 5, 4}, true)
	t2([AFDO_VERSION_LENGTH]int{5, 5, 5, 5, 4}, [AFDO_VERSION_LENGTH]int{5, 5, 5, 5, 5}, false)
	t2([AFDO_VERSION_LENGTH]int{5, 5, 5, 5, 5}, [AFDO_VERSION_LENGTH]int{5, 5, 5, 3, 5}, true)
	t2([AFDO_VERSION_LENGTH]int{5, 5, 5, 3, 5}, [AFDO_VERSION_LENGTH]int{5, 5, 5, 5, 5}, false)
	t2([AFDO_VERSION_LENGTH]int{5, 5, 5, 5, 5}, [AFDO_VERSION_LENGTH]int{5, 5, 2, 5, 5}, true)
	t2([AFDO_VERSION_LENGTH]int{5, 5, 2, 5, 5}, [AFDO_VERSION_LENGTH]int{5, 5, 5, 5, 5}, false)
	t2([AFDO_VERSION_LENGTH]int{5, 5, 5, 5, 5}, [AFDO_VERSION_LENGTH]int{5, 1, 5, 5, 5}, true)
	t2([AFDO_VERSION_LENGTH]int{5, 1, 5, 5, 5}, [AFDO_VERSION_LENGTH]int{5, 5, 5, 5, 5}, false)
	t2([AFDO_VERSION_LENGTH]int{5, 5, 5, 5, 5}, [AFDO_VERSION_LENGTH]int{0, 5, 5, 5, 5}, true)
	t2([AFDO_VERSION_LENGTH]int{0, 5, 5, 5, 5}, [AFDO_VERSION_LENGTH]int{5, 5, 5, 5, 5}, false)
}

func mockGSList(t *testing.T, urlmock *mockhttpclient.URLMock, bucket, path string, items map[string]string) {
	fakeUrl := fmt.Sprintf("https://www.googleapis.com/storage/v1/b/%s/o?alt=json&delimiter=&pageToken=&prefix=%s&projection=full&versions=false", bucket, url.PathEscape(path))
	resp := gsObjectList{
		Kind:  "storage#objects",
		Items: []gsObject{},
	}
	for item, timestamp := range items {
		resp.Items = append(resp.Items, gsObject{
			Kind:                    "storage#object",
			Id:                      bucket + path + item,
			SelfLink:                bucket + path + item,
			Name:                    item,
			Bucket:                  bucket,
			Generation:              "1",
			Metageneration:          "1",
			ContentType:             "application/octet-stream",
			TimeCreated:             timestamp,
			Updated:                 timestamp,
			StorageClass:            "MULTI_REGIONAL",
			TimeStorageClassUpdated: timestamp,
			Size:      "12345",
			Md5Hash:   "dsafkldkldsaf",
			MediaLink: fakeUrl + item,
			Crc32c:    "eiekls",
			Etag:      "lasdfklds",
		})
	}
	respBytes, err := json.MarshalIndent(resp, "", "  ")
	assert.NoError(t, err)
	urlmock.MockOnce(fakeUrl, mockhttpclient.MockGetDialogue(respBytes))
}

func TestAFDORepoManager(t *testing.T) {
	testutils.LargeTest(t)

	ctx, wd, gb, _, urlmock, cleanup := setupAfdo(t)
	defer cleanup()
	g := setupFakeGerrit(t, wd)

	// Initial update, everything up-to-date.
	mockGSList(t, urlmock, AFDO_GS_BUCKET, AFDO_GS_PATH, map[string]string{
		afdoRevBase: afdoTimeBase,
	})
	rm, err := NewAFDORepoManager(ctx, wd, gb.RepoUrl(), "master", depot_tools.GetDepotTools(t, ctx), g, "fake.server.com", urlmock.Client())
	assert.NoError(t, err)
	assert.Equal(t, mockUser, rm.User())
	assert.Equal(t, afdoRevBase, rm.LastRollRev())
	assert.Equal(t, afdoRevBase, rm.NextRollRev())
	fch, err := rm.FullChildHash(ctx, rm.LastRollRev())
	assert.NoError(t, err)
	assert.Equal(t, fch, rm.LastRollRev())
	rolledPast, err := rm.RolledPast(ctx, afdoRevPrev)
	assert.NoError(t, err)
	assert.True(t, rolledPast)
	rolledPast, err = rm.RolledPast(ctx, afdoRevBase)
	assert.NoError(t, err)
	assert.True(t, rolledPast)
	rolledPast, err = rm.RolledPast(ctx, afdoRevNext)
	assert.NoError(t, err)
	assert.False(t, rolledPast)
	assert.Nil(t, rm.PreUploadSteps())
	assert.Equal(t, 0, rm.CommitsNotRolled())

	// There's a new version.
	mockGSList(t, urlmock, AFDO_GS_BUCKET, AFDO_GS_PATH, map[string]string{
		afdoRevBase: afdoTimeBase,
		afdoRevNext: afdoTimeNext,
	})
	assert.NoError(t, rm.Update(ctx))
	assert.Equal(t, afdoRevBase, rm.LastRollRev())
	assert.Equal(t, afdoRevNext, rm.NextRollRev())
	rolledPast, err = rm.RolledPast(ctx, afdoRevPrev)
	assert.NoError(t, err)
	assert.True(t, rolledPast)
	rolledPast, err = rm.RolledPast(ctx, afdoRevBase)
	assert.NoError(t, err)
	assert.True(t, rolledPast)
	rolledPast, err = rm.RolledPast(ctx, afdoRevNext)
	assert.NoError(t, err)
	assert.False(t, rolledPast)
	assert.Equal(t, 1, rm.CommitsNotRolled())

	// Upload a CL.
	issue, err := rm.CreateNewRoll(ctx, rm.LastRollRev(), rm.NextRollRev(), emails, cqExtraTrybots, false)
	assert.NoError(t, err)
	assert.Equal(t, issueNum, issue)
	msg, err := ioutil.ReadFile(path.Join(rm.(*afdoRepoManager).parentDir, ".git", "COMMIT_EDITMSG"))
	assert.NoError(t, err)
	from, to, err := autoroll.RollRev(strings.Split(string(msg), "\n")[0], func(h string) (string, error) {
		return rm.FullChildHash(ctx, h)
	})
	assert.NoError(t, err)
	assert.Equal(t, afdoRevBase, from)
	assert.Equal(t, afdoRevNext, to)
}
