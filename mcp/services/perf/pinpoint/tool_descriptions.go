package pinpoint

// Tools
const bisectionDescription = `
This will trigger a Pinpoint bisect job, and return both the job id and the url
to the Pinpoint job started. Bisect (bisection) aims to find a culprit for a
regression (or anomaly) that was detected for a particular benchmark and
platform in a particular range of time which is defined by a start (base git
hash) and end (experimental git hash). It runs a binary search against
candidates, until it finds the change that caused the regression.
`

const pairwiseDescription = `
This will trigger a Pinpoint try job, and return both the job id and the url to
the Pinpoint job started.Try (or try job) is an (A/B test) and will trigger a
Pinpoint. This try job compares the performance of Chrome on a particular
benchmark on a particular platform at two different the performance of Chrome
on a particular benchmark on a particular platform at two different points in
time, defined by the base (or control, or A) and the experiment (or the
treatment, or B). When testing a set of changes (also referred to as a Gerrit
patchset or Gerrit change), the base_git_hash and experiment_git_hash are
usually the same value, with the experiment_patch applied so that, in the
context of an A/B test, A is compared against A + patchset to measure the impact
of the patchset in isolation.
`

// Tool Arguments

const benchmarkDescription = `
The benchmark of interest to run. For example, press benchmarks commonly refer
to one of \"speedometer3.crossbench\", \"jetstream2.crossbench\" or
\"motionmark1.3.crossbench\". One job (bisect or try) can be associated with
only one benchmark (and one story). For the full list of supported benchmarks,
use the command \"perf list benchmarks\". This is a required field.
`

const baseGitHashDescription = `
A git hash SHA (either full or short form) of the first commit required for a
Pinpoint job. In the context of bisection, this is the starting commit of the
range you'd like to bisect. For try jobs (or A/B experiments), this is the base
(also refered to the control, or A) of that A/B comparison. Pinpoint currently
only accepts git hashes based on the Chromium repository (chromium/src) for
the base. For example, 2d98fb0e9f9f0fdb24c78d8fd29a8a0b029852ba or 2d98fb0 for
full or short form respectively from
https://chromium.googlesource.com/chromium/src/. This is a required field.
`

const storyDescription = `
A story refers to a set of actions run by the benchmark. In other words, a
subset of tests run within the benchmark. For example, for the press benchmarks
(\"speedometer3.crossbench\", \"jetstream2.cro왜ssbench\" or
\"motionmark1.3.crossbench\") the story is \"default\". This is a required
field.
`

const botConfigurationDescription = `
The bot configuration refers to a tester (or test builder) on the Perf waterfall
to run the benchmarks on. A tester maps 1:1 with a specific device type. For
example, the tester \"mac-m3-pro-perf\" refers to testing a benchmark on Mac
M3 Pro devices.
`

const experimentGitHashDescription = `
The git hash SHA (either full or short form) of the other commit for a Pinpoint
job. In the context of bisection, this is the ending commit of the range you'd
like to bisect. For a try job (or an A/B test), this is the experiment (also
referred to as the treatment, or B). Pinpoint currently only accepts git hashes
based on the Chromium repository (chromium/src) for the base. For example,
2d98fb0e9f9f0fdb24c78d8fd29a8a0b029852ba or 2d98fb0 for full or short form
respectively from https://chromium.googlesource.com/chromium/src/. This is a
required field.
`

const iterationDescription = `
The number of iterations to run the benchmark. Higher iterations usually yield
more granular benchmark results, but at the tradeoff of consuming (and holding
onto) additional resources. This value defaults to 10. The recommmended value
for try jobs is 20. A value over 50 will be rejected. This is an optional field.
`

const newPinpointDescription = `
Turn on this flag to target the new Pinpoint implementation. New Pinpoint
refers to the implementation from the buildbot repository, built on Skia
infrastructure. This defaults to false.
`

const baseRevisionDescription = `
The revision number, or the commit position, for the base of the Pinpoint job.
This is converted to a git hash using crrev.com/. Because this is the base, in
a try job (or A/B test) this value is used as the base (or the control, or A)
for that experiment. For a bisect job, it defines the starting point to run the
bisection for when searching for a culprit. If the base git hash is already
provided, this value will be ignored. One of base revision flag or base git
hash is required to run a try or bisect job.
`

const experimentRevisionDescription = `
The revision number, or the commit position, for the experiment of the Pinpoint
job. This is converted to a git hash using crrev.com/. Because this is the base,
in a try job (or A/B test) this value is used as the experiment (or B) for that
experiment. For a bisect job, it defines the starting point to run the bisection
for when searching for a culprit. If the experiment git hash is already
provided, this value will be ignored. One of experiment revision flag or
experiment git hash is required to run a try or bisect job.
`

const userDescription = `
The user, usually in the form of an email, that is making the Pinpoint job
request. This user email will be tied to the triggered job, so that on the
Pinpoint dashboard, the triggered job will appear under the user's list of jobs.
`

const bugIDDescription = `
The Buganizer ID to associate with this execution. If a Buganizer ID is
provided, that ticket will be updated with the status and results of the
Pinpoint job.
`

const projectDescription = `
The project associated with the Pinpoint job. Usually, this is \"chromium\"
and is the default value.
`
const basePatchDescription = `
A Gerrit change ID that can be applied as a patch to the base git hash (or
revision). This means that when Chromium is built, it will checkout Chromium at
the base_git_hash defined, and then apply this Gerrit patchset before building
Chromium. The patch should be the full URL to the change, not just the patch id.
For example, https://chromium-review.googlesource.com/c/chromium/src/+/6527568.
If there's a patch, it should be appended to the very end. For example, if the
user wants to apply patchset 2, an example would look like
https://chromium-review.googlesource.com/c/chromium/src/+/6527568/2.
`

const experimentPatchDescription = `
A Gerrit change ID that can be applied as a patch to the experiment git hash
(or revision). This means that when Chromium is built, it will checkout Chromium
at the experiment_git_hash defined, and then apply this Gerrit patchset before
building Chromium. The patch should be the full URL to the change, not just the
patch id. For example,
https://chromium-review.googlesource.com/c/chromium/src/+/6527568.
If there's a patch, it should be appended to the very end. For example, if the
user wants to apply patchset 2, an example would look like
https://chromium-review.googlesource.com/c/chromium/src/+/6527568/2.
`
