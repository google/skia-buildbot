// Store Flaky Time Ranges.
package store

import (
	"context"
	"fmt"
	"time"

	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/suggester/go/dsconst"
	"google.golang.org/api/iterator"
)

type TimeRange struct {
	Begin time.Time
	End   time.Time
}

type Flaky map[string][]*TimeRange

type timeRangeStored struct {
	Begin   time.Time
	End     time.Time
	BotName string
	Open    bool
}

func CreateOrUpdateFlaky(botname string, begin, end time.Time, open bool) error {
	begin = begin.UTC()
	end = end.UTC()
	// Look for an existing timeRangeStored that is open. If none-found
	// then create a new entry (if !open).
	defer metrics2.FuncTimer().Stop()

	ctx := context.Background()
	query := ds.NewQuery(dsconst.FLAKY_RANGES).Filter("Open =", true).Filter("BotName =", botname)
	slice_stored := []*timeRangeStored{}
	keys, err := ds.DS.GetAll(ctx, query, &slice_stored)
	if len(slice_stored) == 1 {
		stored := slice_stored[0]
		stored.Begin = begin
		stored.End = end
		stored.Open = open
		_, err = ds.DS.Put(ctx, keys[0], stored)
		if err != nil {
			return fmt.Errorf("Failed to update time range %v: %s", stored, err)
		}
	} else if len(slice_stored) == 0 {
		stored := &timeRangeStored{
			Begin:   begin,
			End:     end,
			BotName: botname,
			Open:    open,
		}
		_, err = ds.DS.Put(ctx, ds.NewKey(dsconst.FLAKY_RANGES), stored)
		if err != nil {
			return fmt.Errorf("Failed to create time range %v: %s", stored, err)
		}
	} else {
		return fmt.Errorf("Got wrong number of matches for Open query: %s", botname)
	}

	return nil
}

// The End for each open TimeRange will be now.
// The value for since must be a positive duration.
func ReadFlaky(since time.Duration, now time.Time) (Flaky, error) {
	now = now.UTC()
	defer metrics2.FuncTimer().Stop()
	ret := Flaky{}
	ctx := context.Background()

	// Find all the timeRangeStored's that are within the given time range.
	timeSince := now.Add(-1 * since)
	query := ds.NewQuery(dsconst.FLAKY_RANGES).Filter("End >=", timeSince).Order("End")
	it := ds.DS.Run(ctx, query)
	row := &timeRangeStored{}
	for {
		_, err := it.Next(row)
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, fmt.Errorf("Failed loading flaky ranges: %s", err)
		}
		end := row.End
		if row.Open {
			end = now
		}
		tr := &TimeRange{
			Begin: row.Begin,
			End:   end,
		}
		if _, ok := ret[row.BotName]; !ok {
			ret[row.BotName] = []*TimeRange{tr}
		} else {
			ret[row.BotName] = append(ret[row.BotName], tr)
		}
	}
	return ret, nil
}
