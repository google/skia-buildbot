package testutils

import (
	"strings"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
)

func TestGitSetup(t *testing.T) {
	SkipIfShort(t)
	g := GitInit(t)
	defer g.Cleanup()
	GitSetup(g)

	output, err := exec.RunCwd(g.Dir(), "git", "log", "-n", "6", "--format=format:%H:%P", "HEAD")
	assert.NoError(t, err)
	t.Log(output)
	lines := strings.Split(output, "\n")
	assert.Equal(t, 5, len(lines))

	commits := make([]string, 5)
	cmap := make(map[string][]string, 5)
	for i, l := range lines {
		split := strings.Split(l, ":")
		assert.Equal(t, 2, len(split))
		if len(split[1]) == 0 {
			cmap[split[0]] = []string{}
		} else {
			cmap[split[0]] = strings.Split(split[1], " ")
		}
		commits[i] = split[0]
	}

	c1, c2, c3, c4, c5 := commits[4], commits[3], commits[2], commits[1], commits[0]
	assert.Equal(t, 0, len(cmap[c1]))
	assert.Equal(t, 1, len(cmap[c2]))
	assert.Equal(t, []string{c1}, cmap[c2])
	assert.Equal(t, 1, len(cmap[c3]))
	assert.Equal(t, []string{c2}, cmap[c3])
	assert.Equal(t, 1, len(cmap[c4]))
	assert.Equal(t, []string{c2}, cmap[c4])
	assert.Equal(t, 2, len(cmap[c5]))
	assert.Equal(t, []string{c4, c3}, cmap[c5])
}
