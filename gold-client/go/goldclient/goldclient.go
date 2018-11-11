package goldclient

import (
	"crypto/md5"
	"fmt"
	"image/png"
	"os"

	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
)

type GoldClient struct {
	goldServiceID       string
	uninterestingHashes []string
	buildConf           *BuildConfig
}

func New(goldServiceID string, buildConf *BuildConfig) *GoldClient {
	return &GoldClient{
		goldServiceID:       goldServiceID,
		buildConf:           buildConf,
		uninterestingHashes: []string{},
	}
}

func (g *GoldClient) FetchUninterestingHashes() error {
	// Fetch the uninteresting hashes from the Gold service.
	return nil
}

func (g *GoldClient) UninterestingHashes() []string {
	return g.uninterestingHashes
}

func (g *GoldClient) Result() *GoldResult {
	return nil
}

func (g *GoldClient) Empty() bool {
	return true
}

func (g *GoldClient) UploadImages() error {
	return nil
}

func (g *GoldClient) UploadResult() error {
	return nil
}

func (g *GoldClient) Finish() error {
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
