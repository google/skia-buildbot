package child

import (
	"context"
	"encoding/hex"
	"errors"
	"path"
	"regexp"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/gcs/test_gcsclient"
	"go.skia.org/infra/go/testutils"
)

const (
	revisionID_regex    = "123-46-blahblah"
	revisionID_basename = "123-46-blahblah.tar.gz"
	shortRev            = "123-46"
	bucket              = "my-bucket"
	gcsPath_regex       = "path/to/123-46-blahblah/object.tar.gz"
	gcsPath_basename    = "path/to/123-46-blahblah.tar.gz"
	owner               = "me@google.com"
	url_regex           = "https://my-bucket/path/to/123-46-blahblah/object.tar.gz"
	url_basename        = "https://my-bucket/path/to/123-46-blahblah.tar.gz"
)

var (
	revisionIDRegex = regexp.MustCompile(`path/to/([0-9a-z_-]+)/.*`)
	shortRevRegex   = regexp.MustCompile(`(\d+-\d+)-.*`)
	updatedTs       = time.Unix(1650378340, 0)
	fakeMD5         = "abc123"

	expectRevision_regex = &revision.Revision{
		Id:        revisionID_regex,
		Checksum:  fakeMD5,
		Author:    owner,
		Display:   shortRev,
		Timestamp: updatedTs,
		URL:       url_regex,
	}
	expectRevision_basename = &revision.Revision{
		Id:        revisionID_basename,
		Checksum:  fakeMD5,
		Author:    owner,
		Display:   shortRev,
		Timestamp: updatedTs,
		URL:       url_basename,
	}

	objectAttrs_regex = &storage.ObjectAttrs{
		Name:      gcsPath_regex,
		Owner:     owner,
		Updated:   updatedTs,
		MD5:       mustDecodeHex(fakeMD5),
		MediaLink: url_regex,
	}
	objectAttrs_basename = &storage.ObjectAttrs{
		Name:      gcsPath_basename,
		Owner:     owner,
		Updated:   updatedTs,
		MD5:       mustDecodeHex(fakeMD5),
		MediaLink: url_basename,
	}
)

func mustDecodeHex(str string) []byte {
	rv, err := hex.DecodeString(str)
	if err != nil {
		panic(err)
	}
	return rv
}

func getShortRev(revID string) string {
	matches := shortRevRegex.FindStringSubmatch(revID)
	if len(matches) != 2 {
		return "<short rev regex found no match>"
	}
	return matches[1]
}

func TestGCSChild_ObjectAttrsToRevision_Regex(t *testing.T) {
	c := &gcsChild{
		revisionIDRegex: revisionIDRegex,
		shortRev:        getShortRev,
	}
	rev, err := c.objectAttrsToRevision(objectAttrs_regex)
	require.NoError(t, err)
	require.Equal(t, expectRevision_regex, rev)
}

func TestGCSChild_ObjectAttrsToRevision_Basename(t *testing.T) {
	c := &gcsChild{
		revisionIDRegex: nil, // No revision ID regex; just use the basename.
		shortRev:        getShortRev,
	}
	rev, err := c.objectAttrsToRevision(objectAttrs_basename)
	require.NoError(t, err)
	require.Equal(t, expectRevision_basename, rev)
}

func TestGCSChild_GetRevision_Regex(t *testing.T) {
	mockGCS := &test_gcsclient.GCSClient{}
	c := &gcsChild{
		gcs:             mockGCS,
		gcsPath:         gcsPath_regex,
		revisionIDRegex: revisionIDRegex,
		shortRev:        getShortRev,
	}

	// In this case, the ID was extracted out of the path as opposed to being a
	// full path component.  Therefore, we don't have a GCS path which exactly
	// matches.
	gcsObjectPath := path.Join(c.gcsPath, revisionID_regex)
	mockGCS.On("GetFileObjectAttrs", testutils.AnyContext, gcsObjectPath).Return(nil, errors.New("not found"))

	call := mockGCS.On("AllFilesInDirectory", testutils.AnyContext, c.gcsPath, mock.Anything).Return(nil)
	call.RunFn = func(args mock.Arguments) {
		_ = args.Get(2).(func(*storage.ObjectAttrs) error)(objectAttrs_regex)
	}

	rev, err := c.GetRevision(context.Background(), revisionID_regex)
	require.NoError(t, err)
	require.Equal(t, expectRevision_regex, rev)
}

func TestGCSChild_GetRevision_Basename(t *testing.T) {
	mockGCS := &test_gcsclient.GCSClient{}
	c := &gcsChild{
		gcs:      mockGCS,
		gcsPath:  gcsPath_basename,
		shortRev: getShortRev,
	}

	// In this case, the ID was extracted out of the path as opposed to being a
	// full path component.  Therefore, we don't have a GCS path which exactly
	// matches.
	gcsObjectPath := path.Join(c.gcsPath, revisionID_basename)
	mockGCS.On("GetFileObjectAttrs", testutils.AnyContext, gcsObjectPath).Return(objectAttrs_basename, nil)

	rev, err := c.GetRevision(context.Background(), revisionID_basename)
	require.NoError(t, err)
	require.Equal(t, expectRevision_basename, rev)
}

func TestGCSChild_LogRevisions(t *testing.T) {
	mockGCS := &test_gcsclient.GCSClient{}
	c := &gcsChild{
		gcs:           mockGCS,
		gcsPath:       gcsPath_basename,
		getGCSVersion: func(rev *revision.Revision) (gcsVersion, error) { return &testGcsVersion{rev.Id}, nil },
		shortRev:      getShortRev,
	}

	attrA := &storage.ObjectAttrs{
		Name:    "path/to/123-46-blahblah.tar.gz",
		Updated: updatedTs,
	}
	revA := &revision.Revision{
		Id:        "123-46-blahblah.tar.gz",
		Display:   "123-46",
		Timestamp: attrA.Updated,
	}
	attrB := &storage.ObjectAttrs{
		Name:    "path/to/123-47-blahblah.tar.gz",
		Updated: updatedTs.Add(time.Duration(1)),
	}
	revB := &revision.Revision{
		Id:        "123-47-blahblah.tar.gz",
		Display:   "123-47",
		Timestamp: attrB.Updated,
	}
	attrC := &storage.ObjectAttrs{
		Name:    "path/to/123-48-blahblah.tar.gz",
		Updated: updatedTs.Add(time.Duration(2)),
	}
	revC := &revision.Revision{
		Id:        "123-48-blahblah.tar.gz",
		Display:   "123-48",
		Timestamp: attrC.Updated,
	}

	call := mockGCS.On("AllFilesInDirectory", testutils.AnyContext, c.gcsPath, mock.Anything).Return(nil)
	call.RunFn = func(args mock.Arguments) {
		fn := args.Get(2).(func(*storage.ObjectAttrs) error)
		_ = fn(attrA)
		_ = fn(attrB)
		_ = fn(attrC)
	}

	revs, err := c.LogRevisions(context.Background(), revA, revA)
	require.NoError(t, err)
	require.Nil(t, revs)

	revs, err = c.LogRevisions(context.Background(), revA, revB)
	require.NoError(t, err)
	require.Equal(t, revs, []*revision.Revision{revB})

	revs, err = c.LogRevisions(context.Background(), revA, revC)
	require.NoError(t, err)
	require.Equal(t, revs, []*revision.Revision{revC, revB})
}

func TestGCSChild_GetAllRevisions(t *testing.T) {
	mockGCS := &test_gcsclient.GCSClient{}
	c := &gcsChild{
		gcs:             mockGCS,
		gcsPath:         gcsPath_regex,
		getGCSVersion:   func(rev *revision.Revision) (gcsVersion, error) { return &testGcsVersion{rev.Id}, nil },
		revisionIDRegex: revisionIDRegex,
		shortRev:        getShortRev,
	}

	objectAttrs_regexOther := &storage.ObjectAttrs{
		Name:      "path/to/123-47-blahblah/object.tar.gz",
		Owner:     owner,
		Updated:   updatedTs,
		MD5:       mustDecodeHex(fakeMD5),
		MediaLink: "https://my-bucket/path/to/123-47-blahblah/object.tar.gz",
	}

	expectRevision_regexOther := &revision.Revision{
		Id:        "123-47-blahblah",
		Checksum:  fakeMD5,
		Author:    owner,
		Display:   "123-47",
		Timestamp: updatedTs,
		URL:       "https://my-bucket/path/to/123-47-blahblah/object.tar.gz",
	}

	call := mockGCS.On("AllFilesInDirectory", testutils.AnyContext, c.gcsPath, mock.Anything).Return(nil)
	call.RunFn = func(args mock.Arguments) {
		fn := args.Get(2).(func(*storage.ObjectAttrs) error)
		_ = fn(objectAttrs_regex)
		_ = fn(objectAttrs_regexOther)
	}
	revs, err := c.getAllRevisions(context.Background())
	require.NoError(t, err)
	require.Equal(t, len(revs), 2)
	require.Contains(t, revs, expectRevision_regex)
	require.Contains(t, revs, expectRevision_regexOther)
}

type testGcsVersion struct {
	value string
}

func (v *testGcsVersion) Compare(other gcsVersion) int {
	if v.value == other.(*testGcsVersion).value {
		return 0
	} else if v.value > other.(*testGcsVersion).value {
		return -1
	}
	return 1
}

func (v *testGcsVersion) Id() string {
	return v.value
}

func TestGCSChild_Update_Regex(t *testing.T) {
	mockGCS := &test_gcsclient.GCSClient{}
	c := &gcsChild{
		gcs:             mockGCS,
		gcsPath:         gcsPath_regex,
		getGCSVersion:   func(rev *revision.Revision) (gcsVersion, error) { return &testGcsVersion{rev.Id}, nil },
		revisionIDRegex: revisionIDRegex,
		shortRev:        getShortRev,
	}

	objectAttrsOldRevision := &storage.ObjectAttrs{
		Name:      "path/to/111-11-blahblah/object.tar.gz",
		Owner:     owner,
		Updated:   updatedTs,
		MediaLink: "https://my-bucket/path/to/111-11-blahblah/object.tar.gz",
	}

	call := mockGCS.On("AllFilesInDirectory", testutils.AnyContext, c.gcsPath, mock.Anything).Return(nil)
	call.RunFn = func(args mock.Arguments) {
		fn := args.Get(2).(func(*storage.ObjectAttrs) error)
		_ = fn(objectAttrs_regex)
		_ = fn(objectAttrsOldRevision)
	}

	lastRollRev := &revision.Revision{
		Id: "111-11-blahblah",
	}
	tipRev, notRolledRevs, err := c.Update(context.Background(), lastRollRev)
	require.NoError(t, err)
	require.Equal(t, expectRevision_regex, tipRev)
	require.Equal(t, []*revision.Revision{expectRevision_regex}, notRolledRevs)
}

func TestGCSChild_Update_Basename(t *testing.T) {
	mockGCS := &test_gcsclient.GCSClient{}
	c := &gcsChild{
		gcs:           mockGCS,
		gcsPath:       gcsPath_regex,
		getGCSVersion: func(rev *revision.Revision) (gcsVersion, error) { return &testGcsVersion{rev.Id}, nil },
		shortRev:      getShortRev,
	}

	objectAttrsOldRevision := &storage.ObjectAttrs{
		Name:      "path/to/111-11-blahblah.tar.gz",
		Owner:     owner,
		Updated:   updatedTs,
		MediaLink: "https://my-bucket/path/to/111-11-blahblah.tar.gz",
	}

	call := mockGCS.On("AllFilesInDirectory", testutils.AnyContext, c.gcsPath, mock.Anything).Return(nil)
	call.RunFn = func(args mock.Arguments) {
		fn := args.Get(2).(func(*storage.ObjectAttrs) error)
		_ = fn(objectAttrs_basename)
		_ = fn(objectAttrsOldRevision)
	}

	lastRollRev := &revision.Revision{
		Id: "111-11-blahblah.tar.gz",
	}
	tipRev, notRolledRevs, err := c.Update(context.Background(), lastRollRev)
	require.NoError(t, err)
	require.Equal(t, expectRevision_basename, tipRev)
	require.Equal(t, []*revision.Revision{expectRevision_basename}, notRolledRevs)
}
