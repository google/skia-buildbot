package engine

// Common utility functions used in test files in this package.

// asChan create a channel supplied by the given slice of int64s 's'.
func asChan(s []int64) <-chan int64 {
	ret := make(chan int64)
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

// fromChan returns a slice of all the int64s produced by the channel 'ch'.
func fromChan(ch <-chan int64) []int64 {
	ret := []int64{}
	for v := range ch {
		ret = append(ret, v)
	}
	return ret
}
