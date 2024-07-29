include ../../make/bazel.mk

# Run from go/coverage
run-demo-coverage:
	$(BAZEL) build -c dbg //go/coverage/...
	../../_bazel_bin/go/coverage/demo/coveragedemo_/coveragedemo run
	../../_bazel_bin/go/coverage/coverageserver/coverageserver_/coverageserver run \
		--config_filename=./config/demo.json \

# Run from go/coverage
run-coverage:
	$(BAZEL) build -c dbg //go/coverage/...
	../../_bazel_bin/go/coverage/coverageserver/coverageserver_/coverageserver run \
		--config_filename=./config/coverage.json \

run-cockroachdb-emulators:
	../../scripts/run_emulators/run_emulators start
