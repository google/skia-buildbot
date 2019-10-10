package sharedconfig

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestIngesterJson5Config(t *testing.T) {
	unittest.SmallTest(t)
	conf, err := ConfigFromJson5File("./test-file.json5")
	require.NoError(t, err)
	checkTestConfig(t, conf)
}

func checkTestConfig(t *testing.T, conf *Config) {
	require.Equal(t, "./skia", conf.GitRepoDir)
	require.Equal(t, 4, len(conf.Ingesters))
	require.Equal(t, 15*time.Minute, conf.Ingesters["gold"].RunEvery.Duration)
	require.Equal(t, 100, conf.Ingesters["gold"].NCommits)
	require.Equal(t, []*DataSource{{"chromium-skia-gm", "dm-json-v1"},
		{"skia-infra-gm", "dm-json-v1"}}, conf.Ingesters["gold"].Sources)
	require.Equal(t, "", conf.Ingesters["gold-trybot"].Sources[0].Bucket)
}
