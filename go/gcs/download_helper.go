package gcs

import (
	"context"
	"crypto/sha1"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"runtime"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// DownloadHelper provides convenience methods for downloading binaries by SHA1
// sum.
type DownloadHelper struct {
	bucket  string
	s       *storage.Client
	subdir  string
	workdir string
}

// NewDownloadHelper returns a DownloadHelper instance.
func NewDownloadHelper(s *storage.Client, gsBucket, gsSubdir, workdir string) *DownloadHelper {
	return &DownloadHelper{
		bucket:  gsBucket,
		s:       s,
		subdir:  gsSubdir,
		workdir: workdir,
	}
}

// Download downloads the given binary from Google Storage.
func (d *DownloadHelper) Download(name, hash string) error {
	sklog.Infof("Downloading new binary for %s...", name)
	filepath := path.Join(d.workdir, name)
	object := hash
	if d.subdir != "" {
		object = d.subdir + "/" + object
	}
	resp, err := d.s.Bucket(d.bucket).Object(object).NewReader(context.Background())
	if err != nil {
		return skerr.Wrapf(err, "Download helper can't get reader for %s", name)
	}
	if err := util.WithWriteFile(filepath, func(w io.Writer) error {
		_, err := io.Copy(w, resp)
		return skerr.Wrap(err)
	}); err != nil {
		return skerr.Wrapf(err, "Download helper failed to download %s", name)
	}
	if runtime.GOOS != "windows" {
		return skerr.Wrap(os.Chmod(filepath, 0755))
	}
	return nil
}

// MaybeDownload downloads the given binary from Google Storage if necessary.
func (d *DownloadHelper) MaybeDownload(name, hash string) error {
	filepath := path.Join(d.workdir, name)
	info, err := os.Stat(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			return d.Download(name, hash)
		} else {
			return skerr.Wrapf(err, "Failed to stat %s", filepath)
		}
	}
	info, err = os.Stat(filepath)
	if err != nil {
		return skerr.Wrapf(err, "Failed to stat %s", filepath)
	}
	if info.Mode() != 0755 {
		sklog.Infof("Binary %s is not executable.", filepath)
		return d.Download(name, hash)
	}

	var contents []byte
	contents, err = ioutil.ReadFile(filepath)
	if err != nil {
		return skerr.Wrapf(err, "Failed to read %s", filepath)
	}
	sha1sum := sha1.Sum(contents)
	sha1str := fmt.Sprintf("%x", sha1sum)
	if sha1str != hash {
		sklog.Infof("Binary %s is out of date:\nExpect: %s\nGot:    %s", filepath, hash, sha1str)
		return d.Download(name, hash)
	}
	return nil
}

// Close should be called when finished with the DownloadHelper.
func (d *DownloadHelper) Close() error {
	return d.s.Close()
}
