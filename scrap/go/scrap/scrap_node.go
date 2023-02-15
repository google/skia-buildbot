// Package scrap defines the scrap types and functions on them.
package scrap

import (
	"fmt"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

// scrapNode represents a single scrap in a tree of scraps
// with parent/child relationships.
type scrapNode struct {
	Name     string // A reasonably short human friendly scrap name.
	Scrap    ScrapBody
	Children []scrapNode
}

// createScrapNodeNames will create unique names for each node in a tree
// where each name is:
//
//  1. Unique within the node tree.
//  2. Can be appended to a C++ or JavaScript variable. For example a name
//     of "12af" could be appended to "image" to be "image12af".
func createScrapNodeNames(node *scrapNode, nextNodeID *int) {
	for i := 0; i < len(node.Children); i++ {
		createScrapNodeNames(&node.Children[i], nextNodeID)
	}

	node.Name = fmt.Sprintf("%d", *nextNodeID)
	(*nextNodeID) += 1
}

// getImageURLs will return an array of all unique image URLs used by the
// scrap node and all child nodes. The slice of URLs is guaranteed to returned
// in a stable order for an unchanged tree.
func (n *scrapNode) getImageURLs() ([]string, error) {
	var ret []string

	if n.Scrap.Type != SKSL {
		return ret, nil
	}
	for _, child := range n.Children {
		urls, err := child.getImageURLs()
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		// The number of unique images in a shader node tree is not expected
		// to be large, so a simple inseration method is sufficient.
		for _, url := range urls {
			if !util.In(url, ret) {
				ret = append(ret, url)
			}
		}
	}
	fullUrl, err := getSkSLImageURL(n.Scrap)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if !util.In(fullUrl, ret) {
		ret = append(ret, fullUrl)
	}
	return ret, nil
}
