package processor

/*
   Utility for processing large amounts of data on an ongoing basis.
*/

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	re = regexp.MustCompile("^[A-Za-z0-9_]+")
)

// ProcessFn is used to process a single chunk of data.
type ProcessFn func(context.Context, time.Time, time.Time) error

// Processor is a struct used for processing data in chunks by time.
type Processor struct {
	// The earliest time we will ingest.
	BeginningOfTime time.Time

	// Ingestion will take place in chunks of this size.
	ChunkSize time.Duration

	// Name of the processor. Must consist of only alphanumeric characters
	// and underscores.
	Name string

	// How often to run processing.
	Frequency time.Duration

	// This function will be called to process a single chunk. ProcessFn
	// will be called for overlapping ranges repeatedly and should be able
	// to avoid reprocessing the same data, or be fast enough that it
	// doesn't matter.
	ProcessFn ProcessFn

	// Once all data from BeginningOfTime has been processed, we will
	// (re)process data from the given Window up to the current time.
	Window time.Duration

	// Working directory.
	Workdir string

	// Backing file for persisting processedUpTo across restarts.
	file string

	// The time up to which data has been processed.
	processedUpTo time.Time
}

// process one chunk of data.
func (p *Processor) process(ctx context.Context, start, end time.Time) error {
	sklog.Infof("Processing %s for range (%s, %s)", p.Name, start, end)
	if err := p.ProcessFn(ctx, start, end); err != nil {
		return err
	}
	p.processedUpTo = end
	return util.WriteGobFile(p.file, &p.processedUpTo)
}

// run is called periodically. It either catches up to the given window or just
// processes the current window.
func (p *Processor) run(ctx context.Context, now time.Time) error {
	start := p.processedUpTo.Add(-p.Window)
	if p.BeginningOfTime.After(start) {
		start = p.BeginningOfTime
	}
	return util.IterTimeChunks(start, now, p.ChunkSize, func(start, end time.Time) error {
		return p.process(ctx, start, end)
	})
}

// init validates and initializes the Processor.
func (p *Processor) init() error {
	// Validate the Processor.
	if util.TimeIsZero(p.BeginningOfTime) {
		return fmt.Errorf("BeginningOfTime is required.")
	}
	if p.ChunkSize == 0 {
		return fmt.Errorf("ChunkSize is required.")
	}
	if p.Name == "" {
		return fmt.Errorf("Name is required.")
	}
	if !re.MatchString(p.Name) {
		return fmt.Errorf("Name can only contain alphanumeric characters and underscores.")
	}
	if p.Frequency == 0 {
		return fmt.Errorf("Frequency is required.")
	}
	if p.ProcessFn == nil {
		return fmt.Errorf("ProcessFn is required.")
	}
	if p.Window == 0 {
		return fmt.Errorf("Window is required.")
	}
	if p.Workdir == "" {
		return fmt.Errorf("Workdir is required.")
	}

	// Initialize the Processor.
	p.file = path.Join(p.Workdir, p.Name)
	p.processedUpTo = p.BeginningOfTime
	return util.MaybeReadGobFile(p.file, &p.processedUpTo)
}

// Start initiates processing on a periodic timer. If the Processor has not run
// before, all of the data from the beginning of time is processed.
func (p *Processor) Start(ctx context.Context) error {
	if err := p.init(); err != nil {
		return err
	}
	sklog.Infof("Starting at: %s", p.processedUpTo)

	// Start running.
	lv := metrics2.NewLiveness("last_successful_processor_run", map[string]string{
		"name": p.Name,
	})
	go util.RepeatCtx(p.Frequency, ctx, func() {
		if err := p.run(ctx, time.Now()); err != nil {
			sklog.Errorf("Failed to run Processor: %s", err)
		} else {
			lv.Reset()
		}
	})
	return nil
}
