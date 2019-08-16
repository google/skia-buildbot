package mocks

import (
	"bytes"
	"io/ioutil"
	"os"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

// MockResultFileLocationFromFile makes a mock ResultFileLocation using the
// passed in file contents
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
	md5, err := util.MD5FromReader(file, &buf)
	if err != nil {
		return nil, skerr.Wrapf(err, "unable to compute MD5 hash of %s", f)
	}

	mrf := &ResultFileLocation{}

	mrf.On("Name").Return(f).Maybe()
	mrf.On("MD5").Return(md5).Maybe()
	mrf.On("Open").Return(ioutil.NopCloser(&buf), nil).Maybe()
	mrf.On("Content").Return(buf.Bytes()).Maybe()
	mrf.On("TimeStamp").Return(fileInfo.ModTime().Unix()).Maybe()
	return mrf, nil
}
