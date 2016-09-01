Skia Task Scheduler
===================

This directory contains code for a custom Swarming task scheduler used by Skia's
bots.

### Motivation ###
Skia's automated testing involves sets of tasks which depend on one another,
for example compiling code and then running tests on a particular platform.
These tasks are performed on Skia's bots via Swarming. We need a way to
intelligently schedule these Swarming tasks to optimally utilize the machines
in our test lab. In particular, we want to keep up with incoming commits to the
Skia repo by testing multiple commits as part of a single task, and then during
idle time run tests at commits which were previously batched to increase the
granularity of our test data. Additionally, the scheduler must keep track of the
directed acyclic graph of tasks for each commit.

## Design ##
At a high level, the scheduler first generates a set of all tasks which could
possibly be scheduled, filters out tasks which cannot run (eg. due to
unsatisfied dependencies) or should not run (eg. we've already run it), then
assigns a score for each candidate task based on the value added by running that
task (eg. how many new commits it tests, or how large a batch it bisects). It
then sorts the tasks by score to form a queue. When a swarming bot is free, the
scheduler triggers the highest-scoring task candidate which matches the
Swarming dimensions of the bot. Detailed design can be found here:
https://docs.google.com/document/d/12DzzmeDBDomNxTWWtHCRIfj6MoB8Yvw4v5horGuJPek/edit
and here:
https://docs.google.com/document/d/1tKlBi0reIKo6ActxN8TQY-4t80uQCJXv_CW9WVWG5w8/edit
