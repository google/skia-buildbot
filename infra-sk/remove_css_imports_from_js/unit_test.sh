#!/bin/sh

TESTDATA=infra-sk/remove_css_imports_from_js/testdata

# diff returns a non-zero exit code if the files are not identical.
diff $TESTDATA/expected_output.js $TESTDATA/actual_output.js
