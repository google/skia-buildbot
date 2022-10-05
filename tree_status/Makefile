include ../make/bazel.mk

generate:
	go generate ./go/...

build: generate
	$(BAZEL) build --config=mayberemote ...

serve:
	@echo "Bazel doesn't have a serve-all option."
	@echo "For the module under development, try running something like:"
	@echo "bazel run modules/machine-server-sk:demo_page_server"

run-local-instance: build
	# Run "bazel build //tree_status/..." after making any frontend changes
	# to pick up the change without restarting the golang server.
	../_bazel_bin/tree_status/go/tree_status/tree_status_/tree_status --local --resources_dir ../_bazel_bin/tree_status/pages/development --repo=test --repo=skia

frontend:
	# Makes sure all the frontend pages and tests compile.
	$(BAZEL) build --config=mayberemote modules/... pages/...

release: build
	$(BAZEL) run --config=mayberemote //tree_status:push_tree_status_container

push: release
	pushk tree_status

push_internal: release
	pushk --only-cluster skia-corp tree_status

testgo:
	go test ./go/...
