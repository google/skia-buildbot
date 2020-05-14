package bug

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/cid"
)

func TestExpand(t *testing.T) {
	unittest.SmallTest(t)

	c := &cid.CommitDetail{
		URL: "https://skia.googlesource.com/skia/+show/d261e1075a93677442fdf7fe72aba7e583863664",
	}
	clusterLink := "https://perf.skia.org/t/?begin=1498332791&end=1498528391&subset=flagged"
	message := "noise"
	buglink := Expand("https://example.com/?link={cluster_url}&commit={commit_url}&message={message}", clusterLink, c, message)
	assert.Equal(t, "https://example.com/?link=https%3A%2F%2Fperf.skia.org%2Ft%2F%3Fbegin%3D1498332791%26end%3D1498528391%26subset%3Dflagged&commit=https%3A%2F%2Fskia.googlesource.com%2Fskia%2F%2Bshow%2Fd261e1075a93677442fdf7fe72aba7e583863664&message=noise", buglink)
}
