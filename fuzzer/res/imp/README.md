# Fuzzer Polymer Components

**fuzzer-stacktrace-sk:** A visualization of a stacktrace.
Defaults to showing the top 8 rows.  Click to expand entire thing.

**fuzzer-collapse-details-sk:** Shows the details of a set of fuzzes.
For now, only binary ones are supported, but the api fuzzes shouldn't be too hard to add.
If extra details are supplied (in binaryReports),
the view can be clicked to expand and show the individual stack traces.

## Viewing the Demos:

Because of security restrictions, you cannot just open up the demo pages,
you must find this directory with a terminal/shell and run:
```
make && make run
```
