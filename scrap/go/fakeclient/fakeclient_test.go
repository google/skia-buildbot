package fakeclient

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/scrap/go/scrap"
)

func TestLoadScrap_NameExists_ReturnsBody(t *testing.T) {

	fc := New(map[string]scrap.ScrapBody{
		"alpha": {Body: "alpha body"},
		"beta":  {Body: "beta body"},
	})
	ctx := context.Background()
	b, err := fc.LoadScrap(ctx, "does not matter", "alpha")
	require.NoError(t, err)
	assert.Equal(t, b.Body, "alpha body")

	b, err = fc.LoadScrap(ctx, "does not matter", "beta")
	require.NoError(t, err)
	assert.Equal(t, b.Body, "beta body")
}

func TestLoadScrap_NameDoesNotExist_ReturnsError(t *testing.T) {

	fc := New(map[string]scrap.ScrapBody{})
	ctx := context.Background()
	_, err := fc.LoadScrap(ctx, "does not matter", "alpha")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no scrap")
}

func TestCreateScrap_UsesTypeAndBodyForHash(t *testing.T) {

	fc := New(map[string]scrap.ScrapBody{
		"alpha": {Body: "alpha body"},
	})
	ctx := context.Background()
	// Try with some different types and bodies to make sure the hashes are different
	id, err := fc.CreateScrap(ctx, scrap.ScrapBody{
		Type: "cherry",
		Body: "durian",
	})
	require.NoError(t, err)
	assert.Equal(t, string(id.Hash), "ba87cf17b61b1677f06f2417e29778eef512176e96419ab42627dd0a78644aeb")
	assert.Len(t, fc.scraps, 2)

	id, err = fc.CreateScrap(ctx, scrap.ScrapBody{
		Type: "cumin",
		Body: "durian",
	})
	require.NoError(t, err)
	assert.Equal(t, string(id.Hash), "9f3848a05ff5732f791e3a94dbbd604449f77ae8a336ada42e32e3f9c1b0e664")
	assert.Len(t, fc.scraps, 3)

	id, err = fc.CreateScrap(ctx, scrap.ScrapBody{
		Type: "cherry",
		Body: "dates",
	})
	require.NoError(t, err)
	assert.Equal(t, string(id.Hash), "a17d1be6a093213ec4523350a050caa8dfa78099c673c44af1bfe0df2a913f35")
	assert.Len(t, fc.scraps, 4)

	assertHasScrapsWithNames(t, fc,
		"9f3848a05ff5732f791e3a94dbbd604449f77ae8a336ada42e32e3f9c1b0e664",
		"a17d1be6a093213ec4523350a050caa8dfa78099c673c44af1bfe0df2a913f35",
		"alpha",
		"ba87cf17b61b1677f06f2417e29778eef512176e96419ab42627dd0a78644aeb")
}

func TestDeleteScrap_Exists_RemovedFromList(t *testing.T) {

	fc := New(map[string]scrap.ScrapBody{
		"alpha": {Body: "alpha body"},
		"beta":  {Body: "beta body"},
	})
	assertHasScrapsWithNames(t, fc, "alpha", "beta")

	ctx := context.Background()
	err := fc.DeleteScrap(ctx, "does not matter", "beta")
	require.NoError(t, err)

	assertHasScrapsWithNames(t, fc, "alpha")
}

func TestDeleteScrap_NameDoesNotExist_ReturnError(t *testing.T) {

	fc := New(map[string]scrap.ScrapBody{
		"alpha": {Body: "alpha body"},
	})
	ctx := context.Background()
	err := fc.DeleteScrap(ctx, "does not matter", "beta")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no scrap")
	assertHasScrapsWithNames(t, fc, "alpha")
}

func assertHasScrapsWithNames(t *testing.T, client *FakeClient, names ...string) {
	actualNames, err := client.ListNames(context.Background(), "does not matter, it will list them all")
	require.NoError(t, err)
	assert.Equal(t, names, actualNames)
}
