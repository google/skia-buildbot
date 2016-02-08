# Fuzzer Polymer Components

**fuzzer-info-sk:** Loads detailed fuzz reports from server and
shows how many broke at various files/functions/lines.  It automatically sorts the files it shows
based on the number of filtered fuzzes (based on an include list and exclude list).
Uses fuzzer-collapse-file-sk.

**fuzzer-collapse-file-sk:** Shows how many fuzzes broke at various functions in a given file.
It automatically sorts the functions it shows
based on the number of filtered fuzzes  Uses fuzzer-collapse-function-sk.

**fuzzer-collapse-function-sk:** Shows how many fuzzes broke at various lines in a given function.
This element is used because it vastly simplifies the dynamic sum of how many fuzzes are in a file.
Can filter fuzzes based on an include list and exclude list.  Uses fuzzer-collapse-details-sk.

**fuzzer-collapse-details-sk:** Shows the details of a set of fuzzes.
The view can be clicked to expand and show the individual stack traces.
Can filter fuzzes based on an include list and exclude list.  Uses fuzzer-stacktrace-sk.

**fuzzer-stacktrace-sk:** A visualization of a stacktrace.
Defaults to showing the top 8 rows.  Click to expand entire thing.

**fuzzer-filter-sk:**  Allows the user to select an include and an exclude filter list.
It mirrors the choices to the URL bar, in the form of query parameters.  On load, it pulls the
lists from the query params.

**fuzzer-status-sk:** Shows the current commit the fuzzer is working on and if there
are any pending fuzzes.

**fuzzer-summary-sk:** Shows the count of newly found bad and grey fuzzes and the count
of total bad/grey fuzzes for all categories.

## Viewing the Demos:

Because of security restrictions, you cannot just open up the demo pages,
you must find this directory with a terminal/shell and run:
```
make && make run
```
For some of the demos, `sinon-server` is used to supply mock json data.