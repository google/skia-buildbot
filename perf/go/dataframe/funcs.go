package dataframe

func Append(a, b *DataFrame) {
	a.ParamSet.AddParamSet(b.ParamSet)
	for k, v := range b.TraceSet {
		a.TraceSet[k] = v
	}
}
