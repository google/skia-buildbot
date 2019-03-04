package gitstore

import (
	"time"
)

const (
	// initialGraphSize is the assumed starting number of commits in a repository. Just so we
	// don't start with an empty data structure when we building or traversing the graph.
	initialGraphSize = 100000
)

// buildGraph takes a rawGraph (a slice where each element contains a commit hash followed by its
// parents) and returns an instance of CommitGraph.
func buildGraph(rawGraph [][]string, timeStamps []time.Time) *CommitGraph {
	nodeMap := make(map[string]*Node, len(rawGraph))
	for idx, rawNode := range rawGraph {
		hash := rawNode[0]
		nodeMap[hash] = &Node{
			Hash:      hash,
			Parents:   make([]*Node, len(rawNode)-1),
			Timestamp: timeStamps[idx],
		}
	}

	for _, rawNode := range rawGraph {
		for idx, p := range rawNode[1:] {
			nodeMap[rawNode[0]].Parents[idx] = nodeMap[p]
		}
	}

	return &CommitGraph{
		Nodes: nodeMap,
	}
}

// CommitGraph contains commits as Nodes that are connected and thus can be traversed.
// Given a graph a client can retrieve a specific node and traverse the graph like this:
//    // First-parent traversal
//    node := graph.GetNode(someHash)
//    for node != nil {
//        // so something with the commit
//        node = node.Parents[0]
//    }
//
type CommitGraph struct {
	Nodes map[string]*Node
}

// GetNode returns the node in the graph that corresponds to the given hash or nil
func (c *CommitGraph) GetNode(hash string) *Node {
	return c.Nodes[hash]
}

// Node is a node in the commit graph that contains the commit hash, a timestamp and pointers to
// its parent nodes. The first parent is the immediate parent in the same branch (like in Git).
type Node struct {
	Hash      string
	Timestamp time.Time
	Parents   []*Node
}

// DescendantChain returns all nodes in the commit graph in the range of
// (firstAncestor, lastDescendant) where the parameters are both commit hashes.
// 'firstAncestor' can be "" in which case it will return all ancestors of 'lastDescendant'.
// 'lastDescendant' must not be empty and must exist in graph or this will panic.
func (g *CommitGraph) DecendantChain(firstAncestor, lastDescendant string) []*Node {
	curr := g.Nodes[lastDescendant]
	ret := make([]*Node, 0, len(g.Nodes))
	for curr != nil {
		ret = append(ret, curr)
		if (len(curr.Parents) == 0) || (curr.Hash == firstAncestor) {
			break
		}
		curr = curr.Parents[0]
	}

	// Reverse the result
	for idx := 0; idx < len(ret)/2; idx++ {
		rightIdx := len(ret) - 1 - idx
		ret[idx], ret[rightIdx] = ret[rightIdx], ret[idx]
	}
	return ret
}
