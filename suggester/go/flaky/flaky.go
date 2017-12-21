package flaky

import (
	"context"
	"fmt"
	"time"

	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/suggester/go/dsconst"
	"google.golang.org/api/iterator"
)

// Map of bot name to when the flaky comment was added.
type FlakyProvider func() (map[string]time.Time, error)

type TimeRange struct {
	Begin time.Time
	End   time.Time
}

// In returns true if the timestamp fits within the open
// interval of TimeRange, i.e. ts in (Begin, End).
func (t *TimeRange) In(ts time.Time) bool {
	return ts.After(t.Begin) && ts.Before(t.End)
}

type Flaky map[string][]*TimeRange

func (f Flaky) WasFlaky(botname string, ts time.Time) bool {
	if ranges, ok := f[botname]; ok {
		for _, tr := range ranges {
			if tr.In(ts) {
				return true
			}
		}
	}
	return false
}

type FlakyBuilder struct {
	provider FlakyProvider
}

func NewFlakyBuilder(provider FlakyProvider) *FlakyBuilder {
	return &FlakyBuilder{
		provider: provider,
	}
}

type timeRangeStored struct {
	Begin   time.Time
	End     time.Time
	BotName string
	Open    bool
}

func (fb *FlakyBuilder) createOrUpdateFlaky(botname string, begin, end time.Time, open bool) error {
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

func (fb *FlakyBuilder) closeFlaky(botname string) error {
	// Look for an existing timeRangeStored that is open. If none-found
	// then create a new entry (if !open).
	defer metrics2.FuncTimer().Stop()

	ctx := context.Background()
	query := ds.NewQuery(dsconst.FLAKY_RANGES).Filter("Open =", true).Filter("BotName =", botname)
	slice_stored := []*timeRangeStored{}
	keys, err := ds.DS.GetAll(ctx, query, &slice_stored)
	if len(slice_stored) == 1 {
		stored := slice_stored[0]
		stored.Open = false
		_, err = ds.DS.Put(ctx, keys[0], stored)
		if err != nil {
			return fmt.Errorf("Failed to update time range %v: %s", stored, err)
		}
	}

	return nil
}

func (fb *FlakyBuilder) allOpenFlakyBots() (map[string]bool, error) {
	ret := map[string]bool{}
	ctx := context.Background()
	query := ds.NewQuery(dsconst.FLAKY_RANGES).Filter("Open =", true)
	it := ds.DS.Run(ctx, query)
	row := &timeRangeStored{}
	for {
		_, err := it.Next(row)
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, fmt.Errorf("Failed loading flaky ranges: %s", err)
		}
		ret[row.BotName] = false
	}
	return ret, nil
}

func (fb *FlakyBuilder) Update() error {
	flakes, err := fb.provider()
	if err != nil {
		return err
	}
	// First, list all bots that are Open.
	open, err := fb.allOpenFlakyBots()
	if err != nil {
		return err
	}
	stillOpen := map[string]bool{}
	now := time.Now()
	for botname, begin := range flakes {
		stillOpen[botname] = true
		fb.createOrUpdateFlaky(botname, begin, now, true)
	}
	// Close all bots that were Open but aren't in "flakes".
	for botname, _ := range open {
		if !stillOpen[botname] {
			fb.closeFlaky(botname)
		}
	}

	return nil
}

// The End for each open TimeRange will be now.
// The value for since must be a positive duration.
func (fb *FlakyBuilder) Build(since time.Duration, now time.Time) (Flaky, error) {
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
