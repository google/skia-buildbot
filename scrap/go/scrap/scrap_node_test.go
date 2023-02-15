// Package scrap defines the scrap types and functions on them.
package scrap

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateScrapNodeNames_WithChildNodes_NamedViaBFSTraversal(t *testing.T) {
	// It is required that createScrapNodeNames create unique node names
	// in a repeatable way - i.e. the same node is always given the same name.
	// This test verifies that createScrapNodeNames does a BFS traversal
	// when naming scrap nodes.
	root := scrapNode{
		Children: []scrapNode{{}, {}},
	}
	nextNodeID := 1
	createScrapNodeNames(&root, &nextNodeID)
	assert.Equal(t, "3", root.Name)
	require.Equal(t, 2, len(root.Children))
	assert.Equal(t, "1", root.Children[0].Name)
	assert.Equal(t, "2", root.Children[1].Name)
}

func TestGetImageURLs_WithChildNodes_NoDuplicates(t *testing.T) {
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
	urls, err := root.getImageURLs()
	require.NoError(t, err)

	// getImageURLs can return the URLs in any order, so sort the returned
	// slice for test comparision.
	sort.Strings(urls)
	expected := []string{
		"https://shaders.skia.org/img/mandrill.png",
		"https://shaders.skia.org/img/soccer.png",
	}
	require.Equal(t, expected, urls)
}
