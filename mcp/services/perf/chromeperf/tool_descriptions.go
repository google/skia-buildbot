package chromeperf

const listBotConfigurationDescription = `
List all bots that Perf supports for Pinpoint execution. A bot maps one to one
with a machine type. For example, if you want to test on a Pixel 9 Pro or a Mac
M3 pro, the bot would be android-pixel9-pro-perf and mac-m3-pro-perf
respectively. Bot naming conventions are typically {platform}-{device-type}-perf
with a suffix of either -pgo or -cbb to denote whether it's a specific PGO or a
CBB (Cross Browser Benchmarking) variant of the bot.
`

const listBenchmarkDescription = `
Returns a list of all the benchmarks supported for a Pinpoint execution. For
example, speedometer3.crossbench or jetstream2.crossbench.
`

const listStoryDescription = `
A story is an application scenario and a set of actions to run in that scenario.
In the typical Chromium use case, this will be a web page together with actions
like scrolling, clicking, or executing JavaScript. This returns a list of
stories for a particular benchmark.
`
