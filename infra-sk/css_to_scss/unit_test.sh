#!/bin/sh

TESTDATA=infra-sk/css_to_scss/testdata

# diff returns a non-zero exit code if the files are not identical.
diff $TESTDATA/expected_output.scss $TESTDATA/actual_output.scss
