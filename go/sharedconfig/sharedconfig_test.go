package sharedconfig

import (
	"testing"

	assert "github.com/stretchr/testify/require"
)

func TestIngesterConfig(t *testing.T) {
	conf, err := ConfigFromTomlFile("./test-file.toml")
	assert.NoError(t, err)

	assert.Equal(t, "./skia", conf.GitRepoDir)
	assert.Equal(t, 4, len(conf.Ingesters))
	assert.Equal(t, 100, conf.Ingesters["gold"].NCommits)
	assert.Equal(t, []*DataSource{&DataSource{"chromium-skia-gm", "dm-json-v1"},
		&DataSource{"skia-infra-gm", "dm-json-v1"}}, conf.Ingesters["gold"].Sources)

	assert.Equal(t, "", conf.Ingesters["gold-trybot"].Sources[0].Bucket)
}
