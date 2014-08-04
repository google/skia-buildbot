package util

func In(s string, a []string) bool {
	for _, x := range a {
		if x == s {
			return true
		}
	}
	return false
}
