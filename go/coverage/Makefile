include ../../make/bazel.mk

# Check gRPCs:
#  grpc_cli ls 127.0.0.1:8006 -l --channel_creds_type=local
# Use Client:
#  go run coverageclient/main.go
# -addFile | -addBuilder | -addTest | -delete

# [PRE-REQ] CockroachDB to be running on local machine:
# https://www.cockroachlabs.com/docs/v24.1/install-cockroachdb-linux
# Run from go/coverage
run-demo-coverage:
	cd /tmp && cockroach start-single-node --insecure --listen-addr=127.0.0.1 &
	$(BAZEL) build -c dbg //go/coverage/...
	../../_bazel_bin/go/coverage/demo/coveragedemo_/coveragedemo run
	../../_bazel_bin/go/coverage/coverageserver/coverageserver_/coverageserver run \
		--config_filename=demo.json \

# Run from go/coverage
run-coverage:
	$(BAZEL) build -c dbg //go/coverage/...
	../../_bazel_bin/go/coverage/coverageserver/coverageserver_/coverageserver run \
		--config_filename=demo.json \

run-cockroachdb-emulators:
	../../scripts/run_emulators/run_emulators start
