package tracefilter

// TraceFilter provides a struct to create a tree structure
// that can be used to filter traces
type TraceFilter struct {
	traceKey string
	value    string
	children map[string]*TraceFilter
}

// NewTraceFilter creates a new instance of TraceFilter
func NewTraceFilter() *TraceFilter {
	return &TraceFilter{
		traceKey: "HEAD",
		children: map[string]*TraceFilter{},
	}
}

// AddPath adds a path to the tree
func (tf *TraceFilter) AddPath(path []string, traceKey string) {
	if len(path) > 0 {
		pathHead := path[0]
		if _, ok := tf.children[pathHead]; !ok {
			nextNode := TraceFilter{
				traceKey: traceKey,
				value:    pathHead,
				children: map[string]*TraceFilter{},
			}
			tf.children[pathHead] = &nextNode
		}

		// Still more nodes present in the path
		if len(path) > 1 {
			remainingPath := path[1:]
			tf.children[pathHead].AddPath(remainingPath, traceKey)
		}
	}
}

// GetLeafNodeTraceKeys returns the trace keys from the leaf nodes
// This will filter out any traces that have child nodes
func (tf *TraceFilter) GetLeafNodeTraceKeys() []string {
	if len(tf.children) == 0 {
		return []string{tf.traceKey}
	}
	// If there are child nodes, recursively go through all the
	// sub trees to collect the leaf nodes
	childTraceKeys := []string{}
	for _, childNode := range tf.children {
		childTraceKeys = append(childTraceKeys, childNode.GetLeafNodeTraceKeys()...)
	}

	return childTraceKeys
}
