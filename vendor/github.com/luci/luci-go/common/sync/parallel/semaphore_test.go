// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package parallel

import (
	"fmt"
	"sort"
	"sync"
)

// ExampleSemaphore demonstrates Semaphore's usage by processing 20 units of
// data in parallel. It protects the processing with a semaphore Locker to
// ensure that at most 5 units are processed at any given time.
func ExampleSemaphore() {
	sem := make(Semaphore, 5)

	done := make([]int, 20)
	wg := sync.WaitGroup{}
	for i := 0; i < len(done); i++ {
		i := i

		wg.Add(1)
		sem.Lock()
		go func() {
			defer wg.Done()
			defer sem.Unlock()
			done[i] = i
		}()
	}

	wg.Wait()
	sort.Ints(done)
	fmt.Println("Got:", done)

	// Output: Got: [0 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19]
}
