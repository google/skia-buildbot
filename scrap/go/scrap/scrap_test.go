// Package scrap defines the scrap types and functions on them.
package scrap

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"testing"
	"text/template"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/gcs/test_gcsclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

const (
	svgHash          = "f7b0bac33b5f5b3ac86bec9f33c2d1c3ef025a9e4282ca7a8b9bc01e40d40556"
	scrapName        = "@MyScrapName"
	scrapName2       = "@MyScrapName2"
	invalidScrapName = "not-a-valid-scrap-name-missing-@-prefix"
	invalidScrapType = "not-a-valid-scrap-type"
	invalidScrapLang = "not-a-valid-language"
)

var errMyMockError = errors.New("my error returned from mock GCSClient")

// myWriteCloser is a wrapper that turns a bytes.Buffer from an io.Writer to an io.WriteCloser.
type myReadWriteCloser struct {
	bytes.Buffer
}

func (*myReadWriteCloser) Close() error {
	return nil
}

// myErrorOnWriteWriteCloser turns a bytes.Buffer from an io.Writer to an io.WriteCloser that errors when Writing.
type myErrorReadWriteCloser struct {
	bytes.Buffer
}

func (*myErrorReadWriteCloser) Read([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func (*myErrorReadWriteCloser) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func (*myErrorReadWriteCloser) Close() error {
	return nil
}

// myErrorOnWriteWriteCloser turns a bytes.Buffer from an io.Writer to an io.WriteCloser that errors when Closing.
type myErrorOnCloseReadWriteCloser struct {
	bytes.Buffer
}

func (*myErrorOnCloseReadWriteCloser) Close() error {
	return io.ErrShortWrite
}

func TestExpand_HappyPathWithHash_Success(t *testing.T) {
	unittest.SmallTest(t)
	s := &test_gcsclient.GCSClient{}

	var r myReadWriteCloser
	storedScrap := ScrapBody{
		Type: SVG,
		Body: "<svg></svg>",
	}
	err := json.NewEncoder(&r).Encode(storedScrap)
	require.NoError(t, err)

	s.On("FileReader", testutils.AnyContext, fmt.Sprintf("scraps/svg/%s", svgHash)).Return(&r, nil)

	se, err := New(s)
	require.NoError(t, err)

	// Swap out a good template with a known short one.
	oldTemplate := se.templates[CPP][SVG]
	tmpl, err := template.New("").Parse(`const char *s = "{{ .Body }}";`)
	require.NoError(t, err)
	se.templates[CPP][SVG] = tmpl

	// Clean up after the test is done.
	defer func() {
		se.templates[CPP][SVG] = oldTemplate
	}()

	var w bytes.Buffer
	err = se.Expand(context.Background(), SVG, svgHash, CPP, &w)
	require.NoError(t, err)
	require.Equal(t, `const char *s = "<svg></svg>";`, w.String())
}

func TestExpand_TemplateFailsToExpand_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	s := &test_gcsclient.GCSClient{}

	var r myReadWriteCloser
	storedScrap := ScrapBody{
		Type: SVG,
		Body: "<svg></svg>",
	}
	err := json.NewEncoder(&r).Encode(storedScrap)
	require.NoError(t, err)

	s.On("FileReader", testutils.AnyContext, fmt.Sprintf("scraps/svg/%s", svgHash)).Return(&r, nil)

	se, err := New(s)
	require.NoError(t, err)

	// Swap out a good template with a known short one.
	oldTemplate := se.templates[CPP][SVG]
	tmpl, err := template.New("").Parse(`const char *s = "{{ .Body }}";`)
	require.NoError(t, err)
	se.templates[CPP][SVG] = tmpl

	// Clean up after the test is done.
	defer func() {
		se.templates[CPP][SVG] = oldTemplate
	}()

	var w myErrorReadWriteCloser
	err = se.Expand(context.Background(), SVG, svgHash, CPP, &w)
	require.Contains(t, err.Error(), "Failed to expand template")
}

func TestExpand_InvalidLang_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	se, err := New(&test_gcsclient.GCSClient{})
	require.NoError(t, err)

	err = se.Expand(context.Background(), SVG, svgHash, invalidScrapLang, nil)
	require.Contains(t, err.Error(), ErrInvalidLanguage.Error())
}

func TestLoadScrap_HappyPathWithHash_Success(t *testing.T) {
	unittest.SmallTest(t)
	s := &test_gcsclient.GCSClient{}

	var r myReadWriteCloser
	storedScrap := ScrapBody{
		Type: SVG,
		Body: "<svg></svg>",
	}
	err := json.NewEncoder(&r).Encode(storedScrap)
	require.NoError(t, err)

	s.On("FileReader", testutils.AnyContext, fmt.Sprintf("scraps/svg/%s", svgHash)).Return(&r, nil)

	se, err := New(s)
	require.NoError(t, err)
	receivedScrap, err := se.LoadScrap(context.Background(), SVG, svgHash)
	require.NoError(t, err)
	require.Equal(t, storedScrap, receivedScrap)
}

func TestLoadScrap_FileReaderFails_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	s := &test_gcsclient.GCSClient{}

	s.On("FileReader", testutils.AnyContext, fmt.Sprintf("scraps/svg/%s", svgHash)).Return(nil, errMyMockError)

	se, err := New(s)
	require.NoError(t, err)
	_, err = se.LoadScrap(context.Background(), SVG, svgHash)
	require.Contains(t, err.Error(), "Failed to open scrap.")
}

func TestLoadScrap_HappyPathWithName_Success(t *testing.T) {
	unittest.SmallTest(t)
	s := &test_gcsclient.GCSClient{}

	// First call GetName
	var rName myReadWriteCloser
	storedName := Name{
		Hash:        svgHash,
		Description: "Some description of the named scrap.",
	}
	err := json.NewEncoder(&rName).Encode(storedName)
	require.NoError(t, err)

	s.On("FileReader", testutils.AnyContext, fmt.Sprintf("names/svg/%s", scrapName)).Return(&rName, nil)

	// Then load the scrap.
	var rScrap myReadWriteCloser
	storedScrap := ScrapBody{
		Type: SVG,
		Body: "<svg></svg>",
	}
	err = json.NewEncoder(&rScrap).Encode(storedScrap)
	require.NoError(t, err)

	s.On("FileReader", testutils.AnyContext, fmt.Sprintf("scraps/svg/%s", svgHash)).Return(&rScrap, nil)

	se, err := New(s)
	require.NoError(t, err)
	receivedScrap, err := se.LoadScrap(context.Background(), SVG, scrapName)
	require.NoError(t, err)
	require.Equal(t, storedScrap, receivedScrap)
}

func TestLoadScrap_GetNameFails_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	s := &test_gcsclient.GCSClient{}

	// First call GetName
	s.On("FileReader", testutils.AnyContext, fmt.Sprintf("names/svg/%s", scrapName)).Return(nil, errMyMockError)

	se, err := New(s)
	require.NoError(t, err)
	_, err = se.LoadScrap(context.Background(), SVG, scrapName)
	require.Contains(t, err.Error(), "Failed to get hash of name to load.")
}

func TestLoadScrap_InvalidType_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	se, err := New(&test_gcsclient.GCSClient{})
	require.NoError(t, err)
	_, err = se.LoadScrap(context.Background(), invalidScrapType, scrapName)
	require.Contains(t, err.Error(), ErrInvalidScrapType.Error())
}

func TestLoadScrap_InvalidName_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	se, err := New(&test_gcsclient.GCSClient{})
	require.NoError(t, err)
	_, err = se.LoadScrap(context.Background(), SVG, invalidScrapName)
	require.Contains(t, err.Error(), ErrInvalidHash.Error())
}

func TestCreateScrap_HappyPath_Success(t *testing.T) {
	unittest.SmallTest(t)
	s := &test_gcsclient.GCSClient{}

	var w myReadWriteCloser

	s.On("FileWriter", testutils.AnyContext, fmt.Sprintf("scraps/svg/%s", svgHash),
		gcs.FileWriteOptions{
			ContentEncoding: "gzip",
			ContentType:     "application/json",
		}).Return(&w)

	se, err := New(s)
	require.NoError(t, err)
	sentBody := ScrapBody{
		Type: SVG,
		Body: "<svg></svg>",
	}
	id, err := se.CreateScrap(context.Background(), sentBody)
	require.NoError(t, err)

	require.Equal(t, ScrapID{Hash: svgHash}, id)

	// Unzip and decode the written body to confirm it matches what we sent.
	r := bytes.NewReader(w.Bytes())
	rz, err := gzip.NewReader(r)
	require.NoError(t, err)
	var storedBody ScrapBody
	err = json.NewDecoder(rz).Decode(&storedBody)
	require.NoError(t, err)
	require.Equal(t, storedBody, sentBody)
}

func TestCreateScrap_InvalidScrapType_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	s := &test_gcsclient.GCSClient{}
	se, err := New(s)
	require.NoError(t, err)
	sentBody := ScrapBody{
		Type: Type("not-a-known-type"),
		Body: "<svg></svg>",
	}
	_, err = se.CreateScrap(context.Background(), sentBody)
	require.Contains(t, err.Error(), ErrInvalidScrapType.Error())
}

func TestCreateScrap_TooLargeScrap_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	s := &test_gcsclient.GCSClient{}
	se, err := New(s)
	require.NoError(t, err)

	largeBody := make([]byte, maxScrapSize+1)
	for i := range largeBody {
		largeBody[i] = 'a'
	}
	sentBody := ScrapBody{
		Type: SVG,
		Body: string(largeBody),
	}
	_, err = se.CreateScrap(context.Background(), sentBody)
	require.Contains(t, err.Error(), ErrInvalidScrapSize.Error())
}

func TestCreateScrap_FileWriterFailsOnWrite_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	s := &test_gcsclient.GCSClient{}

	var w myErrorReadWriteCloser

	s.On("FileWriter", testutils.AnyContext, fmt.Sprintf("scraps/svg/%s", svgHash),
		gcs.FileWriteOptions{
			ContentEncoding: "gzip",
			ContentType:     "application/json",
		}).Return(&w)

	se, err := New(s)
	require.NoError(t, err)
	sentBody := ScrapBody{
		Type: SVG,
		Body: "<svg></svg>",
	}
	_, err = se.CreateScrap(context.Background(), sentBody)
	require.Contains(t, err.Error(), "Failed to write JSON body.")
}

func TestCreateScrap_FileWriterFailsOnClose_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	s := &test_gcsclient.GCSClient{}

	var w myErrorOnCloseReadWriteCloser

	s.On("FileWriter", testutils.AnyContext, fmt.Sprintf("scraps/svg/%s", svgHash),
		gcs.FileWriteOptions{
			ContentEncoding: "gzip",
			ContentType:     "application/json",
		}).Return(&w)

	se, err := New(s)
	require.NoError(t, err)
	sentBody := ScrapBody{
		Type: SVG,
		Body: "<svg></svg>",
	}
	_, err = se.CreateScrap(context.Background(), sentBody)
	require.Contains(t, err.Error(), "Failed to close GCS Storage writer.")
}

func TestDeleteScrap_HappyPathWithHash_Success(t *testing.T) {
	unittest.SmallTest(t)
	s := &test_gcsclient.GCSClient{}

	s.On("DeleteFile", testutils.AnyContext, fmt.Sprintf("scraps/svg/%s", svgHash)).Return(nil)

	se, err := New(s)
	require.NoError(t, err)
	err = se.DeleteScrap(context.Background(), SVG, svgHash)
	require.NoError(t, err)
}

func TestDeleteScrap_HappyPathWithName_Success(t *testing.T) {
	unittest.SmallTest(t)
	s := &test_gcsclient.GCSClient{}

	// First we call GetName.
	var r myReadWriteCloser
	storedName := Name{
		Hash:        svgHash,
		Description: "Some description of the named scrap.",
	}
	err := json.NewEncoder(&r).Encode(storedName)
	require.NoError(t, err)

	s.On("FileReader", testutils.AnyContext, fmt.Sprintf("names/svg/%s", scrapName)).Return(&r, nil)

	// Then we call DeleteName.
	s.On("DeleteFile", testutils.AnyContext, fmt.Sprintf("names/svg/%s", scrapName)).Return(nil)

	// Then we delete the scrap at the hash.
	s.On("DeleteFile", testutils.AnyContext, fmt.Sprintf("scraps/svg/%s", svgHash)).Return(nil)

	se, err := New(s)
	require.NoError(t, err)
	err = se.DeleteScrap(context.Background(), SVG, scrapName)
	require.NoError(t, err)
	s.AssertExpectations(t)
}

func TestDeleteScrap_GetNameFails_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	s := &test_gcsclient.GCSClient{}

	// First we call GetName.
	s.On("FileReader", testutils.AnyContext, fmt.Sprintf("names/svg/%s", scrapName)).Return(nil, errMyMockError)

	se, err := New(s)
	require.NoError(t, err)
	err = se.DeleteScrap(context.Background(), SVG, scrapName)
	require.Contains(t, err.Error(), "Failed to get hash of name to delete.")
}

func TestDeleteScrap_DeleteNameFails_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	s := &test_gcsclient.GCSClient{}

	// First we call GetName.
	var r myReadWriteCloser
	storedName := Name{
		Hash:        svgHash,
		Description: "Some description of the named scrap.",
	}
	err := json.NewEncoder(&r).Encode(storedName)
	require.NoError(t, err)

	s.On("FileReader", testutils.AnyContext, fmt.Sprintf("names/svg/%s", scrapName)).Return(&r, nil)

	// Then we call DeleteName.
	s.On("DeleteFile", testutils.AnyContext, fmt.Sprintf("names/svg/%s", scrapName)).Return(errMyMockError)

	se, err := New(s)
	require.NoError(t, err)
	err = se.DeleteScrap(context.Background(), SVG, scrapName)
	require.Contains(t, err.Error(), "Failed to delete name.")
}

func TestDeleteScrap_DeleteFileWithHashFails_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	s := &test_gcsclient.GCSClient{}

	s.On("DeleteFile", testutils.AnyContext, fmt.Sprintf("scraps/svg/%s", svgHash)).Return(errMyMockError)

	se, err := New(s)
	require.NoError(t, err)
	err = se.DeleteScrap(context.Background(), SVG, svgHash)
	require.Contains(t, err.Error(), "Failed to delete hash.")
}

func TestDeleteScrap_InvalidType_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	se, err := New(&test_gcsclient.GCSClient{})
	require.NoError(t, err)
	err = se.DeleteScrap(context.Background(), invalidScrapType, svgHash)
	require.Contains(t, err.Error(), ErrInvalidScrapType.Error())
}

func TestDeleteScrap_InvalidName_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	se, err := New(&test_gcsclient.GCSClient{})
	require.NoError(t, err)
	err = se.DeleteScrap(context.Background(), SVG, invalidScrapName)
	require.Contains(t, err.Error(), ErrInvalidHash.Error())
}

func TestPutName_HappyPath_Success(t *testing.T) {
	unittest.SmallTest(t)
	s := &test_gcsclient.GCSClient{}

	var w myReadWriteCloser

	s.On("FileWriter", testutils.AnyContext, fmt.Sprintf("names/svg/%s", scrapName),
		gcs.FileWriteOptions{
			ContentType: "application/json",
		}).Return(&w)

	se, err := New(s)
	require.NoError(t, err)
	sentName := Name{
		Hash:        svgHash,
		Description: "Some description of the named scrap.",
	}
	err = se.PutName(context.Background(), SVG, scrapName, sentName)
	require.NoError(t, err)

	// Decode the written body to confirm it matches what we sent.
	r := bytes.NewReader(w.Bytes())
	var storedName Name
	err = json.NewDecoder(r).Decode(&storedName)
	require.NoError(t, err)
	require.Equal(t, storedName, sentName)
}

func TestPutName_InvalidName_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	se, err := New(&test_gcsclient.GCSClient{})
	require.NoError(t, err)
	sentName := Name{
		Hash: svgHash,
	}
	err = se.PutName(context.Background(), SVG, invalidScrapName, sentName)
	require.Contains(t, err.Error(), ErrInvalidScrapName.Error())
}

func TestPutName_InvalidType_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	se, err := New(&test_gcsclient.GCSClient{})
	require.NoError(t, err)
	sentName := Name{
		Hash: svgHash,
	}
	err = se.PutName(context.Background(), invalidScrapType, scrapName, sentName)
	require.Contains(t, err.Error(), ErrInvalidScrapType.Error())
}

func TestPutName_InvalidHash_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	se, err := New(&test_gcsclient.GCSClient{})
	require.NoError(t, err)
	sentName := Name{
		Hash: "this is not a valid SHA256 hash",
	}
	err = se.PutName(context.Background(), SVG, scrapName, sentName)
	require.Contains(t, err.Error(), ErrInvalidHash.Error())
}

func TestPutName_FileWriterWriteFails_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	s := &test_gcsclient.GCSClient{}

	var w myErrorReadWriteCloser

	s.On("FileWriter", testutils.AnyContext, fmt.Sprintf("names/svg/%s", scrapName),
		gcs.FileWriteOptions{
			ContentType: "application/json",
		}).Return(&w)

	se, err := New(s)
	require.NoError(t, err)
	sentName := Name{
		Hash: svgHash,
	}
	err = se.PutName(context.Background(), SVG, scrapName, sentName)
	require.Contains(t, err.Error(), "Failed to encode JSON.")
}

func TestPutName_FileWriterCloseFails_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	s := &test_gcsclient.GCSClient{}

	var w myErrorOnCloseReadWriteCloser

	s.On("FileWriter", testutils.AnyContext, fmt.Sprintf("names/svg/%s", scrapName),
		gcs.FileWriteOptions{
			ContentType: "application/json",
		}).Return(&w)

	se, err := New(s)
	require.NoError(t, err)
	sentName := Name{
		Hash: svgHash,
	}
	err = se.PutName(context.Background(), SVG, scrapName, sentName)
	require.Contains(t, err.Error(), "Failed to close GCS Storage writer.")
}

func TestGetName_HappyPath_Success(t *testing.T) {
	unittest.SmallTest(t)
	s := &test_gcsclient.GCSClient{}

	var r myReadWriteCloser
	storedName := Name{
		Hash:        svgHash,
		Description: "Some description of the named scrap.",
	}
	err := json.NewEncoder(&r).Encode(storedName)
	require.NoError(t, err)

	s.On("FileReader", testutils.AnyContext, fmt.Sprintf("names/svg/%s", scrapName)).Return(&r, nil)

	se, err := New(s)
	require.NoError(t, err)
	retrievedName, err := se.GetName(context.Background(), SVG, scrapName)
	require.NoError(t, err)
	require.Equal(t, storedName, retrievedName)
}

func TestGetName_FileReaderReadFails_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	s := &test_gcsclient.GCSClient{}

	var r myErrorReadWriteCloser
	s.On("FileReader", testutils.AnyContext, fmt.Sprintf("names/svg/%s", scrapName)).Return(&r, nil)

	se, err := New(s)
	require.NoError(t, err)
	_, err = se.GetName(context.Background(), SVG, scrapName)
	require.Contains(t, err.Error(), "Failed to decode body.")
}

func TestGetName_InvalidScrapName_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	se, err := New(&test_gcsclient.GCSClient{})
	require.NoError(t, err)
	_, err = se.GetName(context.Background(), SVG, invalidScrapName)
	require.Contains(t, err.Error(), ErrInvalidScrapName.Error())
}

func TestGetName_InvalidScrapType_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	se, err := New(&test_gcsclient.GCSClient{})
	require.NoError(t, err)
	_, err = se.GetName(context.Background(), invalidScrapType, scrapName)
	require.Contains(t, err.Error(), ErrInvalidScrapType.Error())
}

func TestGetName_FileReaderFails_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	s := &test_gcsclient.GCSClient{}
	s.On("FileReader", testutils.AnyContext, fmt.Sprintf("names/svg/%s", scrapName)).Return(nil, errMyMockError)
	se, err := New(s)
	require.NoError(t, err)
	_, err = se.GetName(context.Background(), SVG, scrapName)
	require.Contains(t, err.Error(), "Failed to open name.")
}

func TestDeleteName_HappyPath_Success(t *testing.T) {
	unittest.SmallTest(t)
	s := &test_gcsclient.GCSClient{}

	s.On("DeleteFile", testutils.AnyContext, fmt.Sprintf("names/svg/%s", scrapName)).Return(nil)

	se, err := New(s)
	require.NoError(t, err)
	err = se.DeleteName(context.Background(), SVG, scrapName)
	require.NoError(t, err)
}

func TestDeleteName_DeleteFileFails_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	s := &test_gcsclient.GCSClient{}

	s.On("DeleteFile", testutils.AnyContext, fmt.Sprintf("names/svg/%s", scrapName)).Return(errMyMockError)

	se, err := New(s)
	require.NoError(t, err)
	err = se.DeleteName(context.Background(), SVG, scrapName)
	require.Contains(t, err.Error(), "Failed to delete name.")
}

func TestDeleteName_InvalidType_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	se, err := New(&test_gcsclient.GCSClient{})
	require.NoError(t, err)
	err = se.DeleteName(context.Background(), invalidScrapType, scrapName)
	require.Contains(t, err.Error(), ErrInvalidScrapType.Error())
}

func TestDeleteName_InvalidName_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	se, err := New(&test_gcsclient.GCSClient{})
	require.NoError(t, err)
	err = se.DeleteName(context.Background(), SVG, invalidScrapName)
	require.Contains(t, err.Error(), ErrInvalidScrapName.Error())
}

func TestListNames_HappyPath_Success(t *testing.T) {
	unittest.SmallTest(t)
	s := &test_gcsclient.GCSClient{}

	s.On("AllFilesInDirectory", testutils.AnyContext, fmt.Sprintf("names/svg/"), mock.Anything).Run(func(args mock.Arguments) {
		cb := args[2].(func(item *storage.ObjectAttrs) error)
		_ = cb(&storage.ObjectAttrs{Name: scrapName})
		_ = cb(&storage.ObjectAttrs{Name: scrapName2})
	}).Return(nil)

	se, err := New(s)
	require.NoError(t, err)
	list, err := se.ListNames(context.Background(), SVG)
	require.NoError(t, err)
	require.Equal(t, []string{scrapName, scrapName2}, list)
}

func TestListNames_AllFilesInDirectoryFailure_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	s := &test_gcsclient.GCSClient{}

	s.On("AllFilesInDirectory", testutils.AnyContext, fmt.Sprintf("names/svg/"), mock.Anything).Return(errMyMockError)

	se, err := New(s)
	require.NoError(t, err)
	_, err = se.ListNames(context.Background(), SVG)
	require.Contains(t, err.Error(), "Failed to read directory.")
}

func TestListNames_InvalidType_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	se, err := New(&test_gcsclient.GCSClient{})
	require.NoError(t, err)
	_, err = se.ListNames(context.Background(), invalidScrapType)
	require.Contains(t, err.Error(), ErrInvalidScrapType.Error())
}
