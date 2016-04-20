package named

import (
	"fmt"
	"testing"

	"go.skia.org/infra/fiddle/go/store"

	"github.com/stretchr/testify/assert"
)

type namedMock struct {
	lookup map[string]string
}

func (n *namedMock) GetHashFromName(name string) (string, error) {
	if hash, ok := n.lookup[name]; !ok {
		return "", fmt.Errorf("Not found")
	} else {
		return hash, nil
	}
}

func TestNamed(t *testing.T) {
	mock := &namedMock{
		lookup: map[string]string{
			"star":     "cbb8dee39e9f1576cd97c2d504db8eee",
			"bad_hash": "cbb8d",
		},
	}
	names := New(mock)
	_, err := names.DereferenceID("@missing")
	assert.Error(t, err)

	_, err = names.DereferenceID("@bad_hash")
	assert.Error(t, err)

	_, err = names.DereferenceID("cbb8")
	assert.Error(t, err, "Invalid looking fiddle hashes should fail.")

	hash, err := names.DereferenceID("@star")
	assert.NoError(t, err)
	assert.Equal(t, hash, "cbb8dee39e9f1576cd97c2d504db8eee")

	hash, err = names.DereferenceID("cbb8dee39e9f1576cd97c2d504db8eee")
	assert.NoError(t, err)
	assert.Equal(t, hash, "cbb8dee39e9f1576cd97c2d504db8eee")

	_, _, err = names.DereferenceImageID("@missing.pdf")
	assert.Error(t, err)

	_, _, err = names.DereferenceImageID("@bad_hash.pdf")
	assert.Error(t, err)

	_, _, err = names.DereferenceImageID("@star.png")
	assert.Error(t, err, "All .png's should have a prefix of either _raster or _gpu.")

	mediaHash, media, err := names.DereferenceImageID("@star.pdf")
	assert.NoError(t, err)
	assert.Equal(t, mediaHash, "cbb8dee39e9f1576cd97c2d504db8eee")
	assert.Equal(t, media, store.PDF)

	mediaHash, media, err = names.DereferenceImageID("@star_gpu.png")
	assert.NoError(t, err)
	assert.Equal(t, mediaHash, "cbb8dee39e9f1576cd97c2d504db8eee")
	assert.Equal(t, media, store.GPU)

	mediaHash, media, err = names.DereferenceImageID("cbb8dee39e9f1576cd97c2d504db8eee_raster.png")
	assert.NoError(t, err)
	assert.Equal(t, mediaHash, "cbb8dee39e9f1576cd97c2d504db8eee")
	assert.Equal(t, media, store.CPU)
}
