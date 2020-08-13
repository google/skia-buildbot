package channel

func Batch(batchSize int, inCh <-chan interface{}) <-chan []interface{} {
	ch := make(chan interface{}, batchSize)
	go func() {
		for elem := range inCh {
			ch <- elem
		}
		close(ch)
	}()
	outCh := make(chan []interface{})
	go func() {
		buf := []interface{}{}
		for elem := range ch {
			buf = append(buf, elem)
			if len(buf) == batchSize || len(ch) == 0 {
				outCh <- buf
				buf = buf[:0]
			}
		}
		if len(buf) > 0 {
			outCh <- buf
		}
		close(outCh)
	}()
	return outCh
}
