package main

import (
	"context"
	"flag"

	"github.com/davecgh/go-spew/spew"
	"go.skia.org/infra/bisection/go/pinpoint"
	"go.skia.org/infra/bisection/go/read_values"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

// default flag for
// https://pinpoint-dot-chromeperf.appspot.com/job/121752ae6e0000
var (
	modeDefault      = "bisect"
	sampleDefault    = 10
	startDefault     = "60c79f509c12e3afa905fa354a1a450604444f1c"
	endDefault       = "611b5a084486cd6d99a0dad63f34e320a2ebc2b3"
	deviceDefault    = "win-11-perf"
	targetDefault    = "performance_test_suite"
	benchmarkDefault = "v8.browsing_desktop"
	storyDefault     = "browse:social:twitter_infinite_scroll:2018"
	chartDefault     = "v8:gc:cycle:main_thread:young:atomic"
	aggDefault       = "mean"
	magDefault       = 2.0
)

type cliCmd struct {
	jobID      string
	mode       string
	sampleSize int
	startHash  string
	endHash    string
	device     string
	target     string
	benchmark  string
	story      string
	chart      string
	agg        string
	mag        float64
}

func (cli *cliCmd) RegisterFlags() {
	flag.StringVar(&cli.jobID, "id", "", "Job ID. Can input a previous job ID to reuse swarming tasks.")
	flag.StringVar(&cli.mode, "mode", modeDefault, "Job mode. Either try or bisect.")
	flag.IntVar(&cli.sampleSize, "sample-size", sampleDefault, "initial sample size")
	flag.StringVar(&cli.startHash, "start", startDefault, "start or base commit to run on")
	flag.StringVar(&cli.endHash, "end", endDefault, "end or exp commit to run on")
	flag.StringVar(&cli.device, "device", deviceDefault, "device to run on")
	flag.StringVar(&cli.target, "target", targetDefault, "target to run on")
	flag.StringVar(&cli.benchmark, "benchmark", benchmarkDefault, "benchmark to test")
	flag.StringVar(&cli.story, "story", storyDefault, "story to test")
	flag.StringVar(&cli.chart, "chart", chartDefault, "chart/measurement/sub-story to read")
	flag.StringVar(&cli.agg, "dataAgg", aggDefault, "method to aggregate benchmark measurements by. Options are sum, mean, min, max, count, and std.")
	flag.Float64Var(&cli.mag, "magnitude", magDefault, "Raw magnitude expected")
}

func (c *cliCmd) Run() error {
	ctx := context.Background()
	pp, err := pinpoint.New(ctx)
	if err != nil {
		return err
	}
	agg, err := c.getAggregationMethod()
	if err != nil {
		return err
	}
	req := pinpoint.PinpointRunRequest{
		Device:            c.device,
		Benchmark:         c.benchmark,
		Story:             c.story,
		Chart:             c.chart,
		StartCommit:       c.startHash,
		EndCommit:         c.endHash,
		Magnitude:         c.mag,
		AggregationMethod: agg,
	}

	resp, err := pp.Schedule(ctx, req, c.jobID)
	if err != nil {
		return err
	}
	spew.Dump(resp)
	return nil
}

func main() {
	cli := &cliCmd{}
	cli.RegisterFlags()
	flag.Parse()

	err := cli.Run()
	if err != nil {
		sklog.Error(err)
	}
}

func (c *cliCmd) getAggregationMethod() (read_values.AggDataMethodEnum, error) {
	switch c.agg {
	case "count":
		return read_values.Count.AggDataMethod(), nil
	case "max":
		return read_values.Max.AggDataMethod(), nil
	case "mean":
		return read_values.Mean.AggDataMethod(), nil
	case "min":
		return read_values.Min.AggDataMethod(), nil
	case "std":
		return read_values.Std.AggDataMethod(), nil
	case "sum":
		return read_values.Sum.AggDataMethod(), nil
	}
	return nil, skerr.Fmt("Aggregation method %s is not a supported aggregation method", c.agg)
}
