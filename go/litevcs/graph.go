package litevcs

import (
	"time"

	"go.skia.org/infra/go/util"
)

const (
	startGraphSize = 100000
)

func buildGraph(rawGraph [][]string, timeStamps []time.Time) *CommitGraph {
	nodeMap := make(map[string]*Node, startGraphSize)
	for _, rawNode := range rawGraph {
		hash := rawNode[0]
		// sklog.Infof("NODE: %s", hash)
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

type CommitGraph struct {
	Nodes map[string]*Node
}

func (c *CommitGraph) GetNode(hash string) *Node {
	return c.Nodes[hash]
}

type Node struct {
	Hash      string
	Timestamp time.Time
	Parents   []*Node
}

func (n *Node) BranchCommits() []string {
	curr := n
	ret := make([]string, 0, startGraphSize)
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
