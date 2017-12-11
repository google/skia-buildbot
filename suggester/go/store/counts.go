// Store Filename/Bot counts.
package store

import (
	"context"
	"encoding/json"
	"fmt"

	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/suggester/go/dsconst"
	"google.golang.org/api/iterator"
)

type FileCount struct {
	Counts string
}

func WriteTotals(totals map[string]map[string]int) error {
	defer metrics2.FuncTimer().Stop()
	keys := []string{}
	for k, _ := range totals {
		keys = append(keys, k)
	}
	dsKey := ds.NewKey(dsconst.FILE_COUNT)
	fc := &FileCount{}
	ctx := context.Background()
	for _, key := range keys {
		fmt.Printf("Writing %s: %v\n", key, totals[key])
		dsKey.Name = key
		b, err := json.Marshal(totals[key])
		if err != nil {
			return fmt.Errorf("Failed encoding before writing to datastore: %s", err)
		}
		fc.Counts = string(b)
		_, err = ds.DS.Put(ctx, dsKey, fc)
		if err != nil {
			return fmt.Errorf("Failed writing to datastore: %s", err)
		}
	}
	return nil
}

func ReadTotals() (map[string]map[string]int, error) {
	defer metrics2.FuncTimer().Stop()
	ret := map[string]map[string]int{}
	query := ds.NewQuery(dsconst.FILE_COUNT)
	row := map[string]int{}
	fc := &FileCount{}

	ctx := context.Background()
	it := ds.DS.Run(ctx, query)
	for {
		key, err := it.Next(fc)
		fmt.Printf("Iterating: key:%s  %v\n", key, err)
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, fmt.Errorf("Failed loading File Counts: %s", err)
		}
		if err := json.Unmarshal([]byte(fc.Counts), &row); err != nil {
			sklog.Errorf("Malformed JSON for entity type %s at key %s", string(dsconst.FILE_COUNT), key)
		}
		ret[key.Name] = row
	}
	return ret, nil
}
