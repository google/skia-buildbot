package gitstore

import (
	"time"

	"go.skia.org/infra/go/util"
)

const (
	// initialGraphSize is the assumed starting number of commits in a repository. Just so we
	// don't start with an empty data structure when we building or traversing the graph.
	initialGraphSize = 100000
)

// buildGraph takes a rawGraph (a slice where each element contains a commit hash followed by its
// parents) and returns an instance of CommitGraph.
func buildGraph(rawGraph [][]string, timeStamps []time.Time) *CommitGraph {
	nodeMap := make(map[string]*Node, initialGraphSize)
	for _, rawNode := range rawGraph {
		hash := rawNode[0]
		nodeMap[hash] = &Node{
			Hash:    hash,
			Parents: make([]*Node, len(rawNode)-1),
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

// BranchCommits returns all the commit hashes that are first-parents of this node. Assuming
// 'n' is the HEAD of a branch this will return all the hashes for the branch.
func (n *Node) BranchCommits() []string {
	curr := n
	ret := make([]string, 0, initialGraphSize)
	for curr != nil {
		ret = append(ret, curr.Hash)
		if len(curr.Parents) == 0 {
			break
		}
		curr = curr.Parents[0]
	}
	ret = util.Reverse(ret)
	return ret
}
