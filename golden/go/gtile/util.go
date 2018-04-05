package gtile

type StrMap struct {
	Vals    []string
	ValsMap map[string]int32
}

func (s *StrMap) init(capSize int) {
	s.Vals = make([]string, 0, capSize)
	s.ValsMap = make(map[string]int32, capSize)
}

func (s *StrMap) toInt(newVal string) int32 {
	if ret, ok := s.ValsMap[newVal]; ok {
		return ret
	}
	ret := int32(len(s.Vals))
	s.ValsMap[newVal] = ret
	s.Vals = append(s.Vals, newVal)
	return ret
}

func (s *StrMap) intSlice(vals []string, target []int32) []int32 {
	if target == nil {
		target = make([]int32, len(vals))
	}

	for idx, val := range vals {
		target[idx] = s.toInt(val)
	}
	return target
}

func (s *StrMap) intMap(strMap map[string]string) map[int32]int32 {
	ret := make(map[int32]int32, len(strMap))
	for k, v := range strMap {
		ret[s.toInt(k)] = s.toInt(v)
	}
	return ret
}

func (s *StrMap) strMap(intMap map[int32]int32) map[string]string {
	ret := make(map[string]string, len(intMap))
	for k, v := range intMap {
		ret[s.Vals[k]] = s.Vals[v]
	}
	return ret
}

func (s *StrMap) strSlice(intVals []int32) []string {
	ret := make([]string, len(intVals))
	for idx, val := range intVals {
		ret[idx] = s.Vals[val]
	}
	return ret
}
