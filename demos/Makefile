include ../make/bazel.mk

local-instance:
	$(BAZEL) build --config=mayberemote //demos/pages/...
	$(BAZEL) run --config=mayberemote //demos/go/demoserver -- \
        --resources_dir=../_bazel_bin/demos/pages/development \
		--unsynced_repo_path=${SKIA_ROOT} \
		--demos_dir=demos.skia.org/demos \
		--local

local-instance-with-sync:
	$(BAZEL) build --config=mayberemote //demos/pages/...
	$(BAZEL) run --config=mayberemote //demos/go/demoserver -- \
        --resources_dir=../_bazel_bin/demos/pages/development \
		--repo_url=https://skia.googlesource.com/skia \
		--repo_default_branch=main \
		--demos_dir=demos.skia.org/demos \
		--local

release:
	$(BAZEL) run --config=mayberemote //demos:push_demos_container

push: release
	pushk demoserver
