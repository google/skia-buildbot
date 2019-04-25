package gtile

type StrMap struct {
	Vals    []string
	ValsMap map[string]int32
}

func (s *StrMap) Init(capSize int) {
	s.Vals = make([]string, 0, capSize)
	s.ValsMap = make(map[string]int32, capSize)
}

// toInt maps from a string to int32
func (s *StrMap) ToInt(newVal string) int32 {
	if ret, ok := s.ValsMap[newVal]; ok {
		return ret
	}
	ret := int32(len(s.Vals))
	s.ValsMap[newVal] = ret
	s.Vals = append(s.Vals, newVal)
	return ret
}

// intSlice returns a slice of strings with the corresponding int32s
func (s *StrMap) IntSlice(vals []string, target []int32) []int32 {
	if target == nil {
		target = make([]int32, len(vals))
	}

	for idx, val := range vals {
		target[idx] = s.ToInt(val)
	}
	return target
}

// intMap converts map[string]string to map[int]int
func (s *StrMap) IntMap(strMap map[string]string) map[int32]int32 {
	ret := make(map[int32]int32, len(strMap))
	for k, v := range strMap {
		ret[s.ToInt(k)] = s.ToInt(v)
	}
	return ret
}

// strMap converts map[int32]int32 to map[string]string
func (s *StrMap) StrMap(intMap map[int32]int32) map[string]string {
	ret := make(map[string]string, len(intMap))
	for k, v := range intMap {
		ret[s.Vals[k]] = s.Vals[v]
	}
	return ret
}

// strSlice converts the slice of int32s to a slice of strings.
func (s *StrMap) StrSlice(intVals []int32) []string {
	ret := make([]string, len(intVals))
	for idx, val := range intVals {
		ret[idx] = s.Vals[val]
	}
	return ret
}
