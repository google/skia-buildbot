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
<<<<<<< HEAD
	"go.skia.org/infra/go/util"
=======
>>>>>>> suggester wip
	"go.skia.org/infra/predict/go/dsconst"
	"google.golang.org/api/iterator"
)

// FlakyProvider is a type for a func that returns the current list of bots that are flaky and when they were flagged.
type FlakyProvider func() (map[string]time.Time, error)

type TimeRange struct {
	Begin time.Time
	End   time.Time
}

<<<<<<< HEAD
// Contains returns true if the timestamp fits within the half
// interval of TimeRange, i.e. ts in [Begin, End).
func (t *TimeRange) Contains(ts time.Time) bool {
	return (ts.After(t.Begin) || ts.Equal(t.Begin)) && ts.Before(t.End)
=======
// In returns true if the timestamp fits within the open
// interval of TimeRange, i.e. ts in (Begin, End).
func (t *TimeRange) In(ts time.Time) bool {
	sklog.Info(ts, t.Begin, t.End)
	return ts.After(t.Begin) && ts.Before(t.End)
>>>>>>> suggester wip
}

type Flaky map[string][]*TimeRange

// WasFlaky returns true of the given bot was flaky at the given time.
func (f Flaky) WasFlaky(botname string, ts time.Time) bool {
	if ranges, ok := f[botname]; ok {
<<<<<<< HEAD
		for _, tr := range ranges {
			if tr.Contains(ts) {
=======
		sklog.Infof("Testing range for bot %s", botname)
		for _, tr := range ranges {
			if tr.In(ts) {
>>>>>>> suggester wip
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
<<<<<<< HEAD
	Open    bool // Open is true if the bot was still marked flaky the last time we checked.
}

// createOrUpdateFlaky records the given time range as flaky for the bot. If
// the bot already exists then just the End time is updated.
func (fb *FlakyBuilder) createOrUpdateFlaky(ctx context.Context, botname string, begin, end time.Time) error {
=======
	Open    bool
}

func (fb *FlakyBuilder) createOrUpdateFlaky(botname string, begin, end time.Time, open bool) error {
>>>>>>> suggester wip
	begin = begin.UTC()
	end = end.UTC()
	defer metrics2.FuncTimer().Stop()

<<<<<<< HEAD
=======
	ctx := context.Background()
>>>>>>> suggester wip
	query := ds.NewQuery(dsconst.FLAKY_RANGES).
		Filter("Open =", true).
		Filter("BotName =", botname)
	slice_stored := []*timeRangeStored{}
	keys, err := ds.DS.GetAll(ctx, query, &slice_stored)
	if len(slice_stored) == 1 {
		stored := slice_stored[0]
<<<<<<< HEAD
		stored.End = end
		stored.Open = true
=======
		stored.Begin = begin
		stored.End = end
		stored.Open = open
>>>>>>> suggester wip
		_, err = ds.DS.Put(ctx, keys[0], stored)
		if err != nil {
			return fmt.Errorf("Failed to update time range %v: %s", stored, err)
		}
	} else if len(slice_stored) == 0 {
		stored := &timeRangeStored{
			Begin:   begin,
			End:     end,
			BotName: botname,
<<<<<<< HEAD
			Open:    true,
=======
			Open:    open,
>>>>>>> suggester wip
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
<<<<<<< HEAD
func (fb *FlakyBuilder) closeFlaky(ctx context.Context, botname string) error {
	defer metrics2.FuncTimer().Stop()

=======
func (fb *FlakyBuilder) closeFlaky(botname string) error {
	defer metrics2.FuncTimer().Stop()

	ctx := context.Background()
>>>>>>> suggester wip
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
<<<<<<< HEAD
func (fb *FlakyBuilder) allOpenFlakyBots(ctx context.Context) (util.StringSet, error) {
	ret := util.StringSet{}
=======
func (fb *FlakyBuilder) allOpenFlakyBots() (map[string]bool, error) {
	ret := map[string]bool{}
	ctx := context.Background()
>>>>>>> suggester wip
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
<<<<<<< HEAD
		ret[row.BotName] = true
=======
		ret[row.BotName] = false
>>>>>>> suggester wip
	}
	return ret, nil
}

// Update uses the flakyProvider and updates the time ranges of all known flaky bots.
<<<<<<< HEAD
func (fb *FlakyBuilder) Update(ctx context.Context) error {
=======
func (fb *FlakyBuilder) Update() error {
>>>>>>> suggester wip
	sklog.Info("Getting flakes from provider.")
	flakes, err := fb.provider()
	if err != nil {
		return fmt.Errorf("Failed to get current comments: %s", err)
	}
	sklog.Info("Retrieved flakes from provider.")

	// First, list all bots that are Open.
<<<<<<< HEAD
	open, err := fb.allOpenFlakyBots(ctx)
	if err != nil {
		return fmt.Errorf("Failed to get all open flaky bots: %s", err)
	}
	stillOpen := util.StringSet{}
=======
	open, err := fb.allOpenFlakyBots()
	if err != nil {
		return fmt.Errorf("Failed to get all open flaky bots: %s", err)
	}
	stillOpen := map[string]bool{}
>>>>>>> suggester wip
	now := time.Now()

	// Loop over all the flakes and add/update them in the datastore.
	for botname, begin := range flakes {
		stillOpen[botname] = true
<<<<<<< HEAD
		err := fb.createOrUpdateFlaky(ctx, botname, begin, now)
=======
		err := fb.createOrUpdateFlaky(botname, begin, now, true)
>>>>>>> suggester wip
		if err != nil {
			return fmt.Errorf("Failed to update flaky bot %s: %s", botname, err)
		}
	}

	// Close all bots that were Open but aren't in 'flakes'.
<<<<<<< HEAD
	closed := open.Complement(stillOpen)
	for botname, _ := range closed {
		err := fb.closeFlaky(ctx, botname)
		if err != nil {
			return fmt.Errorf("Failed to close flaky bot %s: %s", botname, err)
=======
	for botname, _ := range open {
		if !stillOpen[botname] {
			err := fb.closeFlaky(botname)
			if err != nil {
				return fmt.Errorf("Failed to close flaky bot %s: %s", botname, err)
			}
>>>>>>> suggester wip
		}
	}
	return nil
}

<<<<<<< HEAD
// Build returns a 'Flaky' that covers the given time range.
//
// The End of each open TimeRange will be 'now'.
// The value for since must be a positive duration.
func (fb *FlakyBuilder) Build(ctx context.Context, since time.Duration, now time.Time) (Flaky, error) {
	now = now.UTC()
	defer metrics2.FuncTimer().Stop()
	ret := Flaky{}
=======
// Build returns a 'Flaky' the covers the given time range.
//
// The End of each open TimeRange will be 'now'.
// The value for since must be a positive duration.
func (fb *FlakyBuilder) Build(since time.Duration, now time.Time) (Flaky, error) {
	now = now.UTC()
	defer metrics2.FuncTimer().Stop()
	ret := Flaky{}
	ctx := context.Background()
>>>>>>> suggester wip

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
<<<<<<< HEAD
		ret[row.BotName] = append(ret[row.BotName], tr)
=======
		if _, ok := ret[row.BotName]; !ok {
			ret[row.BotName] = []*TimeRange{tr}
		} else {
			ret[row.BotName] = append(ret[row.BotName], tr)
		}
>>>>>>> suggester wip
	}
	return ret, nil
}
