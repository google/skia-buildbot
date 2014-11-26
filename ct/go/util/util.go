package util

import "fmt"

func GetCTWorkers() []string {
	workers := make([]string, NUM_WORKERS)
	for i := 0; i < NUM_WORKERS; i++ {
		workers[i] = fmt.Sprintf("build%s-m5", i)
	}
	return workers
}
