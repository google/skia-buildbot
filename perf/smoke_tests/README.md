Experimental folder.
bazel test //perf/smoke_tests:perf-page-load_nodejs_test
bazel test //perf/smoke_tests:perf-chrome-public-load-a_nodejs_test
The first test fails, since we don't have an authentication mechanism in place.
