package keysubsetmap

type encodedPair struct {
	keyIdx   int
	valueIdx int
}

//type Value *types.GoldenTrace
type Value string
type Key encodedPair

type Map struct {
	root *node
}

func New() *Map {
	return &Map{
		root: newNode(),
	}
}

type node struct {
	values []Value

	next map[Key]*node
}

func newNode() *node {
	return &node{
		next: map[Key]*node{},
	}
}

func (m *Map) Get(keys []Key) []Value {
	return m.root.Get(keys)
}

func (n *node) Get(keys []Key) []Value {
	if len(keys) == 0 {
		return n.values
	}
	c, ok := n.next[keys[0]]
	if !ok {
		return nil
	}
	return c.Get(keys[1:])
}

func (m *Map) Add(keys []Key, v Value) {
	m.root.Add(keys, v)
}

func (n *node) Add(keys []Key, v Value) {
	n.values = append(n.values, v)
	if len(keys) == 0 {
		return
	}
	for i, k := range keys {
		c, ok := n.next[k]
		if !ok {
			c = newNode()
			n.next[k] = c
		}
		c.Add(keys[i+1:], v)
	}
}
