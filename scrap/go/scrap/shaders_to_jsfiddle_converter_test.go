// Package scrap defines the scrap types and functions on them.
package scrap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWriteInfo_WithChildNodes_UniqueImageIDs(t *testing.T) {
	root := scrapNode{
		Scrap: ScrapBody{
			Type:         SKSL,
			SKSLMetaData: &SKSLMetaData{ImageURL: "/img/soccer.png"},
		},
		Children: []scrapNode{{
			Scrap: ScrapBody{
				Type:         SKSL,
				SKSLMetaData: &SKSLMetaData{ImageURL: "/img/mandrill.png"},
			},
		}, {Scrap: ScrapBody{
			Type:         SKSL,
			SKSLMetaData: &SKSLMetaData{ImageURL: "/img/soccer.png"},
		}}},
	}

	info, err := newWriteInfo(root)
	require.NoError(t, err)
	// Verify that the URL=>id map has no duplicate values.
	ids := make(map[int]bool)
	for _, id := range info.imageIDs {
		ids[id] = true
	}
	assert.Equal(t, 2, len(ids))
	_, ok := ids[1]
	assert.Equal(t, true, ok)
	_, ok = ids[2]
	assert.Equal(t, true, ok)
}

func TestGetNodeImageID_WithChildNodes_TwoIDs(t *testing.T) {
	root := scrapNode{
		Scrap: ScrapBody{
			Type:         SKSL,
			SKSLMetaData: &SKSLMetaData{ImageURL: "/img/soccer.png"},
		},
		Children: []scrapNode{{
			Scrap: ScrapBody{
				Type:         SKSL,
				SKSLMetaData: &SKSLMetaData{ImageURL: "/img/mandrill.png"},
			},
		}, {Scrap: ScrapBody{
			Type:         SKSL,
			SKSLMetaData: &SKSLMetaData{ImageURL: "/img/soccer.png"},
		}}},
	}

	info, err := newWriteInfo(root)
	require.NoError(t, err)
	ids := make(map[int]bool)

	id, err := getNodeImageID(root.Scrap, info)
	require.NoError(t, err)
	ids[id] = true

	id, err = getNodeImageID(root.Children[0].Scrap, info)
	require.NoError(t, err)
	ids[id] = true

	id, err = getNodeImageID(root.Children[1].Scrap, info)
	require.NoError(t, err)
	ids[id] = true
	assert.Equal(t, 2, len(ids))
}
