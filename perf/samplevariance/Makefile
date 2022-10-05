include ../make/bazel.mk

default:
	$(BAZEL) run --config=mayberemote :samplevariance -- > /tmp/yesterday.csv

test:
	$(BAZEL) test --config=mayberemote //perf/samplevariance/...

# An example of how to run over a specific set of JSON files.
example:
	$(BAZEL) run --config=mayberemote :samplevariance -- --prefix=gs://skia-perf/nano-json-v1/2021/05/23/02/