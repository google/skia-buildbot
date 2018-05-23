Fuzzer Production Manual
========================

First make sure you are familiar with the design of fuzzer by reading the
[DESIGN](./DESIGN.md) doc.
A high level overview of the UI is found [here](https://docs.google.com/document/d/1FZZnfEXzuNcjshveX1R35Lp-96-iLJWibNYZ7_WgZjg/edit).

Nomenclature
------------

A single **fuzzer** generates **bad fuzzes** which crash Skia.
These are referred to by their **categories**, e.g. skcodec_mode.
[afl-fuzz](http://lcamtuf.coredump.cx/afl/) is the underlying mechanism that runs the fuzzers.
**Fuzzer**, with a capital 'F', refers to the fuzzing system, the machine or machines running in GCE.

The [design doc](./DESIGN.md) explores these concepts more thoroughly.

Understanding the monitoring dashboard
======================================

Found at https://mon.skia.org/dashboard/db/fuzzer the fuzzer dashboard reports metrics to understand the *liveness* and the *effectiveness* of the fuzzer.
Metrics are usually broken down by fuzzer category.

Liveness Metrics
----------------

 - "Backend processes": The number of go routines dedicated to various tasks.
Metrics starting with "afl-fuzz" represent n processes being used by afl-fuzz.
These numbers should match those laid out in fuzzer-be.service and non-zero.

 - "Queue sizes": The number of fuzzes in the various pieces of the analysis pipeline.
These are usually all 0, except when rolling a fuzzer forward.

 - "New Fuzzes (before deduplication)": The number of new bad fuzzes found by a given fuzzer.
Experimental fuzzers tend to generate many of these, especially when the Fuzzer restarts or has been rolled forward.


Effectiveness Metrics
---------------------

- "Total Paths found by afl-fuzz": Afl-fuzz instruments Skia and is able to tell when it has found a new code path.
This refers to the number of unique code paths found by all afl-fuzz processes working for a fuzzer.
If this number stays below 100 for any fuzzer for a long time (> 1 hour), either the fuzzer isn't working or the code it is testing is very trivial.
One common explanation for the first case is if something is causing the fuzzer to always bail out early w/o doing anything interesting.
Try bench testing the fuzzer with different seeds.

- "Completed Cycles": Afl-fuzz gives its progress in the form of cycles, i.e. one full gambit of randomization patterns.
1 completed cycle constitutes a significant amount of testing.

- "Fuzzes executed per second": This refers to the the fuzzer0, i.e. the master afl-fuzz process.
If a fuzzer is configured to have more than one afl-fuzz process (e.g. most of the binary ones), the master fuzzer is configured to do intelligent fuzzing (i.e. with genetic algorithms),
while the rest do blind fuzzing and occasionally dump their results into the master's queue.
The best fuzzers consistently have over 500 fuzzes per second.
Some fuzzers will occasionally dip below this while afl-fuzz explores some slower code paths, especially after completing some cycles.
If a fuzzer is routinely below this threshold, it may be worth some engineering time to review speeding the fuzzer up.


Rolling the Fuzzer forward
======================================

The fuzzer should be given ample time to explore deep into the Skia codebase.
It is recommended to check the effectiveness metrics on the monitoring page (see above) before rolling forward.
All stable fuzzers should have at least 1 cycle completed, more being better.
All fuzzers should have at least 100 paths found and the total paths graph should be basically flat.
If all stable fuzzers have completed 2 or more cycles and the total paths graph is flat (less than a 1% increase in the last 3 hours), it is probably okay to roll forward.
If the number of cycles is greater than 5 and the total paths graph is flat, it is definitely a good time to roll forward.

Additionally, if the Fuzzer has been working on a version for more than 10 days, it is time to move forward.

Rolling forward with the UI
--------------
On the menu of fuzzer.skia.org, there should be a menu-item to click to "Roll Fuzzer".
This goes to a page like:
https://screenshot.googleplex.com/30yzscvx99N

Paste in the Skia revision to roll forward and click the button.

Rolling forward with the CLI
---------------
There is a simple app in the Skia Buildbot repo called [update_version_to_fuzz](https://github.com/google/skia-buildbot/blob/d4feb7c69fecb31f6a5d97786637cfe794f3b356/fuzzer/go/update_version_to_fuzz/main.go).

Build it with `make update`

Run this like `update_version_to_fuzz --version_to_fuzz deadbeef`

The CLI also takes a `--bucket` param, if you want to upload to the `fuzzer-test` bucket.


Rebooting the Fuzzer
====================
afl-fuzz runs only in memory, so rebooting the Fuzzer will start over any fuzzing progress.
Before deciding to reboot the Fuzzer, look at the guidelines in Rolling the Fuzzer forward.
A recommended practice is to roll the fuzzer forward before or after a reboot; as long as progress will be reset, it won't hurt to get closer to Tip-of-Tree at the same time.

Reboot the fuzzer via the Google Cloud Developer Panel, or by ssh'ing into the bot.
There is no need to stop the fuzzer processes first.

Recreating the Fuzzer GCE instance(s)
=====================================
The source of truth for Fuzzer is in Google Storage - everything will be rebuilt from there.
Following the guidelines of "Rebooting the Fuzzer", recreating the Fuzzer instance(s) is safe.

Rebuild the instances with the [compute_engine_scripts](https://github.com/google/skia-buildbot/tree/d4feb7c69fecb31f6a5d97786637cfe794f3b356/compute_engine_scripts/fuzzer)

Alerts
======

Items below here should include target links from alerts.

full_upload
-----------
This means the uploading of fuzzes from the Fuzzer to Google Storage is a bottleneck.
This could mean that GCS is having a problem (check the logs for 500 errors).
If this is not the case, more go routines should be allocated to upload in fuzzer-be.service.

full_analysis
-------------
This means the analysis of fuzzes on the Fuzzer is a bottleneck.
This could mean that a roll forward has gotten stuck (check the logs).
If this is not the case, more go routines should be allocated to upload in fuzzer-be.service.

useless_fuzzer
--------------
This means a fuzzer has been stuck at less than 20 total paths found (see monitoring).
This probably means there is a glitch in the fuzzers code or more and better seeds need to be used.

stale_version
-------------
The Fuzzer needs to be rolled forward, as it hasn't in 10 days.
See above for instructions.

broken_roll
-------------
A roll forward has taken more than 2 hours.  A typical time for a roll forward is about 20 minutes.
Check the logs for failures.

Adding new Fuzzers
======

[Adding a new binary fuzzer](https://docs.google.com/document/d/1QDX0o8yDdmhbjoudNsXc66iuRXRF5XNNqGnzDzX7c2I/edit#)
[Adding a new API fuzzer](https://docs.google.com/document/d/1e3ikXO7SwoBsbsi1MF06vydXRlXvYalVORaiUuOXk2Y/edit)
