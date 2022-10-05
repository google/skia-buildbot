include ../make/bazel.mk

.PHONY: build
build:
	$(BAZEL) build --config=mayberemote //task_driver/...

.PHONY: release
release: build
	$(BAZEL) run --config=mayberemote //task_driver:push_task_driver_container

.PHONY: push
push: release
	$(BAZEL) run --config=mayberemote //kube/go/pushk -- --message="$(MESSAGE)" --use-temp-checkout task-driver-server

.PHONY: serve
serve:
	@echo "Bazel doesn't have a serve-all option."
	@echo "For the module under development, try running something like:"
	@echo "bazel run //task_scheduler/modules/job-search-sk:demo_page_server"

run-local-instance: build
	# Run "bazel build //task_driver/..." after making any frontend changes
	# to pick up the change without restarting the golang server.
	../_bazel_bin/task_driver/go/task-driver-server/task-driver-server_/task-driver-server --local --project_id=skia-swarming-bots --resources_dir ../_bazel_bin/task_driver/pages/development --bigtable_instance=staging --bigtable_project=skia-public
