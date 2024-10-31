package caching

func ByBlameKey(corpus string) string {
	return corpus + "_byblame"
}

func UnignoredKey(corpus string) string {
	return corpus + "_traces"
}
