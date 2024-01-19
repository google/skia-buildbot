run-local-instance:
	$(BAZEL) build --config=mayberemote  -c dbg //tool/...
	../_bazel_bin/tool/go/srv/srv_/srv \
		--local \
		--port=:8001 \
		--prom_port=:20001 \
		--resources_dir=../_bazel_bin/tool/pages/development/

# Presumes you've got the k8s-config repo checked out, done via
# the checkout-k8s-config Makefile target below.
run-with-local-k8s-config-data:
	$(BAZEL) build --config=mayberemote  -c dbg //tool/...
	../_bazel_bin/tool/go/srv/srv_/srv \
		--local \
		--port=:8001 \
		--prom_port=:20001 \
		--resources_dir=../_bazel_bin/tool/pages/development/ \
		--configs=./k8s-config/tool


run-prod-data:
	$(BAZEL) build --config=mayberemote  -c dbg //tool/...
	../_bazel_bin/tool/go/srv/srv_/srv \
		--local \
		--port=:8001 \
		--prom_port=:20001 \
		--resources_dir=../_bazel_bin/tool/pages/development/ \
		--repo=https://skia.googlesource.com/k8s-config \
		--configs=tool

validate:
	bazelisk run //tool/go/cmd/validate -- --dir=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))/configs

checkout-k8s-config:
	git clone https://skia.googlesource.com/k8s-config

fast-clone-k8s-config:
	git clone --depth=1 https://skia.googlesource.com/k8s-config