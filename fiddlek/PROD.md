Fiddle Production Manual
========================

First make sure you are familiar with the design of fiddle by reading the
[DESIGN](./DESIGN.md) doc.

Alerts
======

Items below here should include target links from alerts.

fiddler_pods
------------

Fiddle should run enough fiddler pods that the number of idle fiddle pods never
gets too low. Check the amount of traffic that fiddle is receiving and if the
traffic is legitimate then increase the number of replicas in the fiddler.yaml
file.

NamedFiddlesFailing
-------------------
Some of the named fiddles checked into the Skia repo have stopped compiling or running. To see what
the compilation errors are and to help with fixes, it is probably easiest to reproduce locally
using the following steps:
  1) Check out Skia <https://skia.org/docs/user/download/>. All following steps should be executed
     from the Skia root.
  2) Sync third party deps by executing `python2 tools/git-sync-deps`
  3) Set up a debug GN build target `bin/gn gen out/Debug`
  4) Re-create the all_examples.cpp `python tools/fiddle/make_all_examples_cpp.py`
  5) Build the fiddle_examples `ninja -C out/Debug fiddle_examples`

Once the fixes have been committed to the Skia repo, they should be synced into fiddle and the
errors should go away.

For runtime errors, it is best to look at the logs of the named-fiddles pod.

Another way to mitigate the alert is to disable the example by putting `#ifdef 0` as the first line
and `#endif` as the last line, then committing.

Key metrics: named_fiddles_errors_in_examples_run

InvalidNamedFiddles
-------------------
Some of the named fiddles cannot be parsed (using //named-fiddles/go/parse). This might mean that
there is a typo in the macro, or a new macro has been introduced and the parsing code needs
updating.

For more information, look at the logs of the named-fiddles pod.

Key metrics: named_fiddles_examples_total_invalid