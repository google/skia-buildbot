Command line application to locally run pinpoint via `pinpoint.Run()`.

## Disclaimer

This application is WIP and intended to locally test the Pinpoint job workflow.

## Usage

- `bazelisk run //bisection/go/cmd` runs the `pinpoint.Run()` with default flags.
  Default flags based off of existing Pinpoint jobs are below.
- `bazelisk run //bisection/go/cmd -- -id=<id>` sets the jobID to `<id>`.
  Pinpoint jobs can recycle the swarming tasks of a previous job with the same job ID.
  This can be useful if you want to quickly navigate through the Pinpoint job workflow
  without having to wait for new swarming tasks to run.

## Default flags

It may be convenient to copy default flags in rather than manually inputting
them into the command line. Here are a few default flag presets based off of
existing Pinpoint jobs:

default flags based off of:
https://pinpoint-dot-chromeperf.appspot.com/job/12e91b146e0000

```
var (
	modeDefault      = "bisect"
	sampleDefault    = 10
	baseDefault      = "ade53cb0512224d12f922be99a2cd81a215c3279"
	expDefault       = "fe7c7f5c993a59ff86a31d740de92174a58acd4e"
	deviceDefault    = "linux-perf"
	targetDefault    = "performance_test_suite"
	benchmarkDefault = "v8.browsing_desktop"
	storyDefault     = "browse:media:pinterest:2018"
	chartDefault     = "v8:gc:cycle:main_thread:full:incremental:sweep"
	aggDefault = "sum"
	magDefault = 11.8
)
```

default flags for
https://pinpoint-dot-chromeperf.appspot.com/job/121752ae6e0000

```
var (
	modeDefault      = "bisect"
	sampleDefault    = 10
	startDefault      = "60c79f509c12e3afa905fa354a1a450604444f1c"
	endDefault       = "611b5a084486cd6d99a0dad63f34e320a2ebc2b3"
	deviceDefault    = "win-11-perf"
	targetDefault    = "performance_test_suite"
	benchmarkDefault = "v8.browsing_desktop"
	storyDefault     = "browse:social:twitter_infinite_scroll:2018"
	chartDefault     = "v8:gc:cycle:main_thread:young:atomic"
	aggDefault       = "mean"
	magDefault       = 2.0
)
```
