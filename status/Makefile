include ../make/bazel.mk

.PHONY: serve
serve:
	@echo "Bazel doesn't have a serve-all option."
	@echo "For the module under development, try running something like:"
	@echo "bazel run //status/modules/autoroller-status-sk:demo_page_server"

testgo:
	$(BAZEL) test --config=mayberemote //status/go/...

testjs:
	$(BAZEL) test --config=mayberemote //status/modules/...

build:
	$(BAZEL) build --config=mayberemote //status/...

release: build
	$(BAZEL) run --config=mayberemote //status:push_status_container

push: release
	$(BAZEL) run --config=mayberemote //kube/go/pushk -- --message="$(MESSAGE)" --use-temp-checkout status

# Run status locally. This is just for convenience in testing.
.PHONY: run-local-instance
run-local-instance: build
	# Run "bazel build //status/..." after making any frontend changes
	# to pick up the change without restarting the golang server.
	../_bazel_bin/status/go/status/status_/status --testing --chrome_infra_auth_jwt=chrome-infra-auth-jwt --bigtable_project=skia-public --bigtable_instance=production --capacity_recalculate_interval=30m --firestore_instance=production --host=localhost --port=:8000 --prom_port=:20000 --repo=https://skia.googlesource.com/skia.git --repo=https://skia.googlesource.com/buildbot.git --repo=https://skia.googlesource.com/lottie-ci.git --repo=https://skia.googlesource.com/skcms.git --resources_dir ../_bazel_bin/status/pages/development --swarming_url=https://chromium-swarm.appspot.com --task_scheduler_url=https://task-scheduler.skia.org --tree_status_base_url=https://tree-status.skia.org