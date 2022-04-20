include ../make/bazel.mk

.PHONY: build
build:
	$(BAZEL) build //gitsync/...

.PHONY: btgit
btgit:
	go install ./go/btgit/...

.PHONY: release
release: build
	$(BAZEL) run //gitsync:push_gitsync_container

.PHONY: push
push: release
	$(BAZEL) run //kube/go/pushk -- --message="$(MESSAGE)" --use-temp-checkout gitsync
