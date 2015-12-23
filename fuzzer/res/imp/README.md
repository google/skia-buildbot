# Fuzzer Polymer Components

**fuzzer-stacktrace-sk:** A visualization of a stacktrace.
Defaults to showing the top 8 rows.  Click to expand entire thing.

**fuzzer-collapse-details-sk:** Shows the details of a set of fuzzes.
For now, only binary ones are supported, but the api fuzzes shouldn't be too hard to add.
If extra details are supplied (in binaryReports),
the view can be clicked to expand and show the individual stack traces.

**fuzzer-collapse-function-sk:** Shows how many fuzzes broke at various lines in a given function.
Uses fuzzer-collapse-details-sk.

**fuzzer-collapse-file-sk:** Shows how many fuzzes broke at various functions in a given file.
Uses fuzzer-collapse-function-sk.

**fuzzer-summary-list-sk:** Loads fuzz summary statistics from server and
shows how many broke at various files/functions/lines.  Uses fuzzer-collapse-file-sk.

**fuzzer-info-sk:** Loads detailed fuzz reports from server and
shows how many broke at various files/functions/lines.  Uses fuzzer-collapse-file-sk.

**fuzzer-status-sk:** Shows the current commit the fuzzer is working on and if there
are any pending fuzzes.

**fuzzer-count-sk:** Shows the count of newly found bad and grey fuzzes and the count
of total bad/grey fuzzes.

## Viewing the Demos:

Because of security restrictions, you cannot just open up the demo pages,
you must find this directory with a terminal/shell and run:
```
make && make run
```
For some of the demos, `sinon-server` is used to supply mock json data.