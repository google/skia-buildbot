default:
	bazel run :samplevariance -- > /tmp/yesterday.csv

test:
	bazel test //perf/samplevariance/...

# An example of how to run over a specific set of JSON files.
example:
	bazel run :samplevariance -- --prefix=gs://skia-perf/nano-json-v1/2021/05/23/02/