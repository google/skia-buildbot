package pyl

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	contents = `
{
"key1": {
  "swarming": {
    "cipd_packages": [
      {
        "id": "pkg1",
        "revision": "rev1",
      }
    ]
  }
},
"key2": {
  "swarming": {
    "cipd_packages": [
      {
        "id": "pkg1",
        "revision": "rev2",
      },
      {
        "id": "pkg2",
        "revision": "rev3",
      },
    ]
  }
},
}
`
)

func TestGet(t *testing.T) {
	test := func(path, expect string) {
		actual, err := Get(contents, path)
		require.NoError(t, err)
		require.Equal(t, expect, actual)
	}
	test("key1.swarming.cipd_packages.id=pkg1.revision", "rev1")
	test("key2.swarming.cipd_packages.id=pkg1.revision", "rev2")
	test("key2.swarming.cipd_packages.id=pkg2.revision", "rev3")
}

func TestSet(t *testing.T) {
	actual, err := Set(contents, "key2.swarming.cipd_packages.id=pkg2.revision", "new-rev")
	require.NoError(t, err)
	require.Equal(t, `
{
"key1": {
  "swarming": {
    "cipd_packages": [
      {
        "id": "pkg1",
        "revision": "rev1",
      }
    ]
  }
},
"key2": {
  "swarming": {
    "cipd_packages": [
      {
        "id": "pkg1",
        "revision": "rev2",
      },
      {
        "id": "pkg2",
        "revision": "new-rev",
      },
    ]
  }
},
}
`, actual)
}

func TestParsePath(t *testing.T) {
	parsed, err := parsePath("key1.swarming.cipd_packages.id=pkg1.revision")
	require.NoError(t, err)
	require.Equal(t, parsed, []*pathElem{
		{
			Key: "key1",
		},
		{
			Key: "swarming",
		},
		{
			Key: "cipd_packages",
		},
		{
			Key:        "id",
			ValueMatch: "pkg1",
		},
		{
			Key: "revision",
		},
	})
}
