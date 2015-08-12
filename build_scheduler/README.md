Skia Build Scheduler
====================

This directory contains code for a custom buildbot scheduler used by Skia's
buildbots.

### Motivation ###
The built-in buildbot schedulers trigger builds at each commit in the repo.
In most cases, if the bots lag behind a little, multiple commits are batched
into a single build. This is good for preventing the bots from falling behind
during the day, but the result is that the blamelists for performance and
correctness results potentially include a lot of commits, which makes
regressions difficult to track down.

Additionally, the bots have large swaths of idle time during the night and on
weekends, and are occasionally idle during the day. They could be using this
time to run additional builds which separate the commits which were batched
together in previous builds.

## Design ##
This build scheduler takes a different approach. It considers the commits
within a time period and generates scores for commit/buildbot pairs (aka
"build candidates") which represent how much value would be obtained by running
a build on each buildbot at each commit. The build candidates are stored in a
sort of priority queue, sorted by their score.

Whenever a given builder is idle, the scheduler takes the highest-scoring build
candidate for that builder from the queue and schedules it using the
BuildBucket API.
