package stages

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

// StageFile represents a file which defines release pipeline stages.
type StageFile struct {
	// DefaultGitRepo is the URL of the git repository from which most images
	// are built. It is used to provide git commit information when an image
	// does not specify its own git repository.
	DefaultGitRepo string `json:"default_git_repo,omitempty"`
	// Images are the images which have release stages defined in this file.
	Images map[string]*Image `json:"images,omitempty"`
}

// Image represents an image within a StageFile.
type Image struct {
	// GitRepo is the URL of the Git repository from which the image is built.
	GitRepo string `json:"git_repo,omitempty"`
	// Stages are the release stages tracked by users of this image.
	Stages map[string]*Stage `json:"stages,omitempty"`
}

// Stage represents a stage of a given Image.
type Stage struct {
	// GitHash is the Git commit hash at which this version of the image was
	// built.
	GitHash string `json:"git_hash"`
	// Digest is the sha256 digest of the image.
	Digest string `json:"digest"`
}

// Decode the given content as a StageFile.
func Decode(content []byte) (*StageFile, error) {
	rv := new(StageFile)
	if err := json.NewDecoder(bytes.NewReader(content)).Decode(&rv); err != nil {
		return nil, skerr.Wrap(err)
	}
	return rv, nil
}

// DecodeFile parses the given file as a StageFile.
func DecodeFile(filepath string) (*StageFile, error) {
	b, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return Decode(b)
}

// Encode the StageFile.
func (f *StageFile) Encode() ([]byte, error) {
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return b, nil
}

// EncodeFile writes the StageFile to a file.
func (f *StageFile) EncodeFile(filepath string) error {
	return skerr.Wrap(util.WithWriteFile(filepath, func(w io.Writer) error {
		dec := json.NewEncoder(w)
		dec.SetIndent("", "  ")
		return dec.Encode(f)
	}))
}

// GitRepoForImage returns the Git repo URL for the given image.
func (f *StageFile) GitRepoForImage(image string) (string, error) {
	img, ok := f.Images[image]
	if !ok {
		return "", skerr.Fmt("image %q does not exist in %s", image, StageFilePath)
	}
	repoURL := img.GitRepo
	if repoURL == "" {
		repoURL = f.DefaultGitRepo
	}
	return repoURL, nil
}
