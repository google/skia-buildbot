include ../make/bazel.mk

run-auth-proxy:
	bazelisk run -c dbg //kube/cmd/auth-proxy -- \
		--prom-port=:20003 \
		--role=editor=google.com \
		--port=:8000 \
		--target_port=:8080 \
		--local \
		--self_sign_localhost_tls \
		--authtype=oauth2 \
		--use_http1

run-perf-mcp:
	bazelisk run --config=remote //mcp/server -- run --service=perf --baseurl=https://localhost:8000

MAKEFLAGS += -j2
run-perf-mcp-auth: run-auth-proxy run-perf-mcp
