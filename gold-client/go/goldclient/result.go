package goldclient

import "io"

type BuildConfig struct {
	Key           map[string]string `json:"key"`
	GitHash       string            `json:"gitHash"`
	Issue         int64             `json:"issue,string"`
	Patchset      int64             `json:"patchset,string"`
	BuildBucketID int64             `json:"buildbucket_build_id,string"`
}

type TestResult struct {
	Key     map[string]string `json:"key"`
	Options map[string]string `json:"options"`
	Digest  string            `json:"md5"`
}

type GoldResult struct {
	*BuildConfig
	Results []*TestResult `json:"results"`
}

func (g *GoldResult) Add(testName, imagePath, digest string, key map[string]string) error {
	return nil
}

func (g *GoldResult) Write(w io.WriteCloser) error {
	return nil
}
