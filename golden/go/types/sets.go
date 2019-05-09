package types

type DigestSet map[Digest]bool

// Keys returns the keys of a DigestSet
func (s DigestSet) Keys() DigestSlice {
	ret := make([]Digest, 0, len(s))
	for v := range s {
		ret = append(ret, v)
	}
	return ret
}

// AddLists adds lists of digests to the DigestSet and returns
// the receiving DigestSet.
func (s DigestSet) AddLists(lists ...[]Digest) DigestSet {
	for _, oneList := range lists {
		for _, item := range oneList {
			s[item] = true
		}
	}
	return s
}

type TestNameSet map[TestName]bool

// Keys returns the keys of a TestNameSet
func (s TestNameSet) Keys() TestNameSlice {
	ret := make([]TestName, 0, len(s))
	for v := range s {
		ret = append(ret, v)
	}
	return ret
}

type TestNameSlice []TestName

func (b TestNameSlice) Len() int           { return len(b) }
func (b TestNameSlice) Less(i, j int) bool { return string(b[i]) < string(b[j]) }
func (b TestNameSlice) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }

type DigestSlice []Digest

func (b DigestSlice) Len() int           { return len(b) }
func (b DigestSlice) Less(i, j int) bool { return string(b[i]) < string(b[j]) }
func (b DigestSlice) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }
