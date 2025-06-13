# services/perf/pinpoint

Pinpoint is a tool that compares the performance of Chrome on a particular bot
and benchmark (performance test) at any two points, defined by git hashes. By
doing that comparison, users of Pinpoint are able to:

1. Given a commit range, find which commit caused a performance regression.
   This action is called bisection.
2. Given two commit positions, directly compare the performance of those two
   points in time. This action is called a try job.

This folder contains all actions pertaining to the tool. The two
actions supported by Pinpoint are bisections and try jobs.

## Bisections

The perf system measures performance of Chrome at intervals. In other words,
every single commit is not measured for performance. As a result, regressions
can be found after X number commits have landed since the last measurement.

When a regression is detected, the point that is measured is marked as an
anomaly. An anomaly is thus associated with one benchmark and one story.

A benchmark is a performance test, which executes a set of stories. A story is
an application scenario and a set of actions to run in that scenario.
In the typical Chromium use case, this will be a web page together with actions
like scrolling, clicking, or executing JavaScript. This returns a list of
stories for a particular benchmark.

Sheriff Configurations are configurations set by each gardening rotation to be
alerted any time an anomaly is generated for the benchmark(s) of interest.
As part of that configuration, thresholds are set, and anytime an anomaly is
generated that meet the sheriff configuration's guidelines, an alert is sent
to the point of contact set on that configuration.

Sheriff Configurations have the option to opt into auto bisections. Bisections,
bisects, or bisect jobs are all Pinpoint attempts used as part of
the gardening to try and find what caused the regression (culprit) between
the last measurement, and the currently measured point. Bisect will run a binary
search-like algorithm, and see at which change (CL) the performance regresses.

Some rotations do not opt in and trigger bisections manually. Thus, the
bisection tool supports providing both a git hash or a revision (commit
poisition). If the latter is provided, we reference cr-rev to parse that back
to a git hash for the bisection workflow.

## Try Jobs

Try jobs are A/B experiments. Bots in Pinpoint map 1:1 to a device type. Try
jobs are executed to see the performance at one git commit against another, for
a particular benchmark on a particular device type.

# Contributing

- tools.go: GetTools() is called by service.go, and must return the list of
  tools supported by Pinpoint. Add any new high level Pinpoint functionality
  as a tool, and arguments in this file.
- tool_descriptions.go: Add all tool, tool argument descriptions here.
  Descriptions, especially for MCP, can get lengthy, so having it separate
  removes clutter.
- client.go: Underliyng implementation of those tool actions should be done
  here.
