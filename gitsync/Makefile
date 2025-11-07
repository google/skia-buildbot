include ../make/bazel.mk

.PHONY: build
build:
	$(BAZEL) build --config=mayberemote //gitsync/...

.PHONY: btgit
btgit:
	go install ./go/btgit/...

.PHONY: release
release: build
	$(BAZEL) run --config=mayberemote //gitsync:push_gitsync_container
