package sharedconfig

import (
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestIngesterJson5Config(t *testing.T) {
	unittest.SmallTest(t)
	conf, err := ConfigFromJson5File("./test-file.json5")
	assert.NoError(t, err)
	checkTestConfig(t, conf)
}

func checkTestConfig(t *testing.T, conf *Config) {
	assert.Equal(t, "./skia", conf.GitRepoDir)
	assert.Equal(t, 4, len(conf.Ingesters))
	assert.Equal(t, 15*time.Minute, conf.Ingesters["gold"].RunEvery.Duration)
	assert.Equal(t, 100, conf.Ingesters["gold"].NCommits)
	assert.Equal(t, []*DataSource{{"chromium-skia-gm", "dm-json-v1"},
		{"skia-infra-gm", "dm-json-v1"}}, conf.Ingesters["gold"].Sources)
	assert.Equal(t, "", conf.Ingesters["gold-trybot"].Sources[0].Bucket)
}
