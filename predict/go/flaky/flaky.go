// flaky is a module for tracking which bots are flaky and for which time periods.
//
// This is more complicated than it seems because Status only tracks if a bot is
// currently marked as flaky, so we need to periodically poll Status for all the
// flaky bots and notice when a bot is no longer flagged as flaky.

package flaky

import (
	"context"
	"fmt"
	"time"

	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/predict/go/dsconst"
	"google.golang.org/api/iterator"
)

// FlakyProvider is a type for a func that returns the current list of bots that are flaky and when they were flagged.
type FlakyProvider func() (map[string]time.Time, error)

type TimeRange struct {
	Begin time.Time
	End   time.Time
}

// Contains returns true if the timestamp fits within the half
// interval of TimeRange, i.e. ts in [Begin, End).
func (t *TimeRange) Contains(ts time.Time) bool {
	return (ts.After(t.Begin) || ts.Equal(t.Begin)) && ts.Before(t.End)
}

type Flaky map[string][]*TimeRange

// WasFlaky returns true of the given bot was flaky at the given time.
func (f Flaky) WasFlaky(botname string, ts time.Time) bool {
	if ranges, ok := f[botname]; ok {
		for _, tr := range ranges {
			if tr.Contains(ts) {
				return true
			}
		}
	}
	return false
}

func (f Flaky) Len() int {
	return len(f)
}

// FlakyBuilder tracks flaky information and builds 'Flaky' instances.
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
	Open    bool // Open is true if the bot was still marked flaky the last time we checked.
}

// createOrUpdateFlaky records the given time range as flaky for the bot. If
// the bot already exists then just the End time is updated.
func (fb *FlakyBuilder) createOrUpdateFlaky(ctx context.Context, botname string, begin, end time.Time) error {
	begin = begin.UTC()
	end = end.UTC()
	defer metrics2.FuncTimer().Stop()

	query := ds.NewQuery(dsconst.FLAKY_RANGES).
		Filter("Open =", true).
		Filter("BotName =", botname)
	slice_stored := []*timeRangeStored{}
	keys, err := ds.DS.GetAll(ctx, query, &slice_stored)
	if len(slice_stored) == 1 {
		stored := slice_stored[0]
		stored.End = end
		stored.Open = true
		_, err = ds.DS.Put(ctx, keys[0], stored)
		if err != nil {
			return fmt.Errorf("Failed to update time range %v: %s", stored, err)
		}
	} else if len(slice_stored) == 0 {
		stored := &timeRangeStored{
			Begin:   begin,
			End:     end,
			BotName: botname,
			Open:    true,
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

// closeFlaky closes an open range for the given bot.
func (fb *FlakyBuilder) closeFlaky(ctx context.Context, botname string) error {
	defer metrics2.FuncTimer().Stop()

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

// allOpenFlakyBots returns all bots that currently have an open flaky range.
func (fb *FlakyBuilder) allOpenFlakyBots(ctx context.Context) (util.StringSet, error) {
	ret := util.StringSet{}
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
		ret[row.BotName] = true
	}
	return ret, nil
}

// Update uses the flakyProvider and updates the time ranges of all known flaky bots.
func (fb *FlakyBuilder) Update(ctx context.Context) error {
	sklog.Info("Getting flakes from provider.")
	flakes, err := fb.provider()
	if err != nil {
		return fmt.Errorf("Failed to get current comments: %s", err)
	}
	sklog.Info("Retrieved flakes from provider.")

	// First, list all bots that are Open.
	open, err := fb.allOpenFlakyBots(ctx)
	if err != nil {
		return fmt.Errorf("Failed to get all open flaky bots: %s", err)
	}
	stillOpen := util.StringSet{}
	now := time.Now()

	// Loop over all the flakes and add/update them in the datastore.
	for botname, begin := range flakes {
		stillOpen[botname] = true
		err := fb.createOrUpdateFlaky(ctx, botname, begin, now)
		if err != nil {
			return fmt.Errorf("Failed to update flaky bot %s: %s", botname, err)
		}
	}

	// Close all bots that were Open but aren't in 'flakes'.
	closed := open.Complement(stillOpen)
	for botname, _ := range closed {
		err := fb.closeFlaky(ctx, botname)
		if err != nil {
			return fmt.Errorf("Failed to close flaky bot %s: %s", botname, err)
		}
	}
	return nil
}

// Build returns a 'Flaky' that covers the given time range.
//
// The End of each open TimeRange will be 'now'.
// The value for since must be a positive duration.
func (fb *FlakyBuilder) Build(ctx context.Context, since time.Duration, now time.Time) (Flaky, error) {
	now = now.UTC()
	defer metrics2.FuncTimer().Stop()
	ret := Flaky{}

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
		ret[row.BotName] = append(ret[row.BotName], tr)
	}
	return ret, nil
}
