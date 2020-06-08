package mocks

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"io/ioutil"
	"os"
	"time"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
)

// MockResultFileLocationFromFile makes a mock ResultFileLocation using the contents of the
// given file.
func MockResultFileLocationFromFile(f string) (*ResultFileLocation, error) {
	fileInfo, err := os.Stat(f)
	if err != nil {
		return nil, skerr.Wrapf(err, "could not stat %s", f)
	}

	// Read file into buffer and calculate the md5 in the process.
	file, err := os.Open(f)
	if err != nil {
		return nil, skerr.Wrapf(err, "could not open %s", f)
	}
	defer util.Close(file)

	var buf bytes.Buffer
	hash, err := util.MD5FromReader(file, &buf)
	if err != nil {
		return nil, skerr.Wrapf(err, "unable to compute MD5 hash of %s", f)
	}

	mrf := &ResultFileLocation{}

	mrf.On("Name").Return(f)
	mrf.On("MD5").Return(hash)
	mrf.On("Open", testutils.AnyContext).Return(ioutil.NopCloser(&buf), nil)
	mrf.On("Content").Return(buf.Bytes())
	mrf.On("TimeStamp").Return(fileInfo.ModTime().Unix())
	return mrf, nil
}

// MockResultFileLocationWithContent returns a mock ResultFileLocation using the given bytes as
// the content.
func MockResultFileLocationWithContent(name string, content []byte, ts time.Time) *ResultFileLocation {
	hash := md5.New().Sum(content)

	mrf := &ResultFileLocation{}
	mrf.On("Name").Return(name)
	mrf.On("MD5").Return(hex.EncodeToString(hash))
	mrf.On("Open", testutils.AnyContext).Return(ioutil.NopCloser(bytes.NewReader(content)), nil)
	mrf.On("Content").Return(content)
	mrf.On("TimeStamp").Return(ts)
	return mrf
}
