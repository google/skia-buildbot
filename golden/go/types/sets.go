package types

type DigestSet map[Digest]bool

// Keys returns the keys of a DigestSet
func (s DigestSet) Keys() []Digest {
	ret := make([]Digest, 0, len(s))
	for v := range s {
		ret = append(ret, v)
	}
	return ret
}

type TestNameSet map[TestName]bool

// Keys returns the keys of a TestNameSet
func (s TestNameSet) Keys() []TestName {
	ret := make([]TestName, 0, len(s))
	for v := range s {
		ret = append(ret, v)
	}
	return ret
}

//TODO(kjlubick): fill in some functionality from string_set

type TestNameSlice []TestName

func (b TestNameSlice) Len() int           { return len(b) }
func (b TestNameSlice) Less(i, j int) bool { return string(b[i]) < string(b[j]) }
func (b TestNameSlice) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }

type DigestSlice []Digest

func (b DigestSlice) Len() int           { return len(b) }
func (b DigestSlice) Less(i, j int) bool { return string(b[i]) < string(b[j]) }
func (b DigestSlice) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }
