package goldclient

import (
	"crypto/md5"
	"fmt"
	"image"
	"image/png"
	"os"

	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
)

type GoldClient interface {
	Test(name string, img image.Image, hash string) error
	Finalize() error
	Pass() bool
}

// Implement the GoldClient interface for a remote Gold server.
type cloudClient struct {
	goldURL             string
	uninterestingHashes []string
	buildConf           *BuildConfig
}

func NewCloudClient(goldServiceID string, buildConf *BuildConfig) GoldClient {
	return &cloudClient{
		goldServiceID:       goldServiceID,
		buildConf:           buildConf,
		uninterestingHashes: []string{},
	}
}

func (g *cloudClient) FetchUninterestingHashes() error {
	// Fetch the uninteresting hashes from the Gold service.
	return nil
}

func (g *cloudClient) UninterestingHashes() []string {
	return g.uninterestingHashes
}

func (g *cloudClient) Result() *GoldResult {
	return nil
}

func (g *cloudClient) Empty() bool {
	return true
}

func (g *cloudClient) UploadImages() error {
	return nil
}

func (g *cloudClient) UploadResult() error {
	return nil
}

func (g *cloudClient) Finish() error {
	return nil
}

func HashFile(fileName string) (string, error) {
	// Load the image
	reader, err := os.Open(fileName)
	if err != nil {
		return "", err
	}
	defer util.Close(reader)

	img, err := png.Decode(reader)
	if err != nil {
		return "", err
	}
	nrgbaImg := diff.GetNRGBA(img)
	return HashImage(nrgbaImg.Pix), nil
}

func HashImage(buf []uint8) string {
	return fmt.Sprintf("%x", md5.Sum(buf))
}
