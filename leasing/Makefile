include ../make/bazel.mk

generate:
	go generate ./go/...

build: generate
	$(BAZEL) build ...

run-local-instance: build
	# Run "bazel build //leasing/..." after making any frontend changes
	# to pick up the change without restarting the golang server.
	../_bazel_bin/leasing/go/leasing/leasing_/leasing --local --resources_dir ../_bazel_bin/leasing/pages/development --project_name skia-public

frontend:
	# Makes sure all the frontend pages and tests compile.
	$(BAZEL) build modules/... pages/...

release: build
	$(BAZEL) run //leasing:push_leasing_container

push: release
	$(BAZEL) run //kube/go/pushk -- --use-temp-checkout leasing
