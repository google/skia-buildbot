package benchmarks

import (
	"math/rand"
	"runtime"
	"strings"
	"testing"

	"go.skia.org/infra/go/testutils/unittest"
)

// This file demonstrates the overhead of calling runtime.Caller and runtime.FuncForPC
// On a 4 core 2017 Thinkpad, the following times were had:
// BenchmarkControl-4                     	10937064	       102 ns/op
// BenchmarkRuntimeCaller_GetFile-4       	 2613914	       439 ns/op
// BenchmarkRuntimeCaller_GetFunction-4   	 2198131	       473 ns/op

func BenchmarkControl(b *testing.B) {
	unittest.ManualTest(b)
	for i := 0; i < b.N; i++ {
		file := getRandomString()
		if len(file) == 1000 { // prevent return value from being optimized away.
			panic("should never happen")
		}
	}
}

func BenchmarkRuntimeCaller_GetFile(b *testing.B) {
	unittest.ManualTest(b)
	for i := 0; i < b.N; i++ {
		_, file, _, ok := runtime.Caller(0)
		if !ok || len(file) == 1000 { // prevent return value from being optimized away.
			panic("should never happen")
		}
	}
}

func BenchmarkRuntimeCaller_GetFunction(b *testing.B) {
	unittest.ManualTest(b)
	for i := 0; i < b.N; i++ {
		pc, _, _, ok := runtime.Caller(0)
		f := runtime.FuncForPC(pc)
		if !ok || len(f.Name()) == 1000 { // prevent return value from being optimized away.
			panic("should never happen")
		}
	}
}

func getRandomString() string {
	n := rand.Intn(40)
	return strings.Repeat("b", n)
}
