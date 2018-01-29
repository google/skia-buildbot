package mass_process

import (
	"errors"
	"fmt"
	"sync"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

/*
	Utilities for reducing many GS objects into fewer.
*/

// Reduction combines multiple objects into one through a series of pairwise
// combinations.
type Reduction interface {
	// Decode the input into the intermediate representation.
	ReadInput([]byte) ([]byte, error)
	// Reduce the two objects in the intermediate representation to a
	// single object in the intermediate representation.
	Reduce([]byte, []byte) ([]byte, error)
	// Encode the result, in the intermediate representation, to its final
	// output format.
	WriteOutput([]byte) ([]byte, error)
}

// Combine a pair of objects in their intermediate format using the given Reduction.
func reducePair(b *storage.BucketHandle, pair []string, inPrefix, outPrefix string, r Reduction) error {
	left, err := ReadObj(b.Object(pair[0]))
	if err != nil {
		return err
	}
	obj := b.Object(GetOutputPath(pair[0], inPrefix, outPrefix))
	if len(pair) == 1 {
		return WriteObj(obj, left)
	}

	right, err := ReadObj(b.Object(pair[1]))
	if err != nil {
		return err
	}

	out, err := r.Reduce(left, right)
	if err != nil {
		return err
	}
	return WriteObj(obj, out)
}

// Pairwise combine all objects from the previous iteration to produce N/2 new
// objects.
func reduceIntermediate(b *storage.BucketHandle, inPrefix, outPrefix string, r Reduction) (int, error) {
	queue := make(chan string)
	pairs := make(chan []string)
	n := 0
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		pair := []string{}
		for path := range queue {
			pair = append(pair, path)
			if len(pair) == 2 {
				pairs <- pair
				pair = []string{}
				n++
			}
		}
		if len(pair) > 0 {
			pairs <- pair
			n++
		}
		close(pairs)
	}()

	var mtx sync.Mutex
	failed := map[string]error{}
	for i := 0; i < NUM_WORKERS; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for pair := range pairs {
				if err := reducePair(b, pair, inPrefix, outPrefix, r); err != nil {
					mtx.Lock()
					failed[pair[0]] = err
					mtx.Unlock()
				}
			}
		}()
	}

	if err := SearchObj(b, inPrefix, queue, -1); err != nil {
		return 0, err
	}
	wg.Wait()
	if len(failed) > 0 {
		errMsg := fmt.Sprintf("Failed to process %d objects:\n", len(failed))
		for k, v := range failed {
			errMsg += fmt.Sprintf("  %s: %s", k, v)
		}
		return 0, errors.New(errMsg)
	}
	return n, nil
}

// Combine all objects using the given Reduction. The general procedure is to
// "decode" the original objects into an intermediate format, then combine all
// of the objects in lg(N) iterations of pairwise combinations, then "encode"
// the single resulting object from the intermediate format into a final format.
func ReduceMany(b *storage.BucketHandle, inPrefix, outPrefix string, r Reduction) error {
	id, err := util.GenerateID()
	if err != nil {
		return err
	}
	intermediatePrefix := fmt.Sprintf(".wip.%s", id)
	prefixTmpl := fmt.Sprintf("%s/%%d/%s", intermediatePrefix, outPrefix)
	i := 0

	// Decode inputs into intermediate format.
	prevPrefix := inPrefix
	nextPrefix := fmt.Sprintf(prefixTmpl, i)
	if err := TransformMany(b, prevPrefix, nextPrefix, r.DecodeInput, -1); err != nil {
		return err
	}
	n := -1
	for n != 1 {
		i++
		prevPrefix = nextPrefix
		nextPrefix = fmt.Sprintf(prefixTmpl, i)
		n, err = reduceIntermediate(b, prevPrefix, nextPrefix, r)
		if err != nil {
			return err
		}
		sklog.Infof("N = %d", n)
	}
	if err := TransformMany(b, nextPrefix, outPrefix, r.EncodeResult, -1); err != nil {
		return err
	}
	return Delete(b, intermediatePrefix)
}
