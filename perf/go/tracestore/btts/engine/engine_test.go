package engine

// Common utility functions used in multiple test files in this package.

// asChan create a channel supplied by the given slice of strings 's'.
func asChan(s []string) <-chan string {
	ret := make(chan string)
	if len(s) == 0 {
		close(ret)
		return ret
	}
	go func() {
		for _, v := range s {
			ret <- v
		}
		close(ret)
	}()
	return ret
}

// fromChan returns a slice of all the strings produced by the channel 'ch'.
func fromChan(ch <-chan string) []string {
	ret := []string{}
	for v := range ch {
		ret = append(ret, v)
	}
	return ret
}
