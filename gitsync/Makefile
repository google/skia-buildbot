include ../make/bazel.mk
include ../kube/kube.mk

.PHONY: build
build:
	$(BAZEL) build //gitsync/...

.PHONY: btgit
btgit:
	go install ./go/btgit/...

.PHONY: release
release: build
	$(BAZEL) run //gitsync:gitsync_container

.PHONY: push
push: release pushk
	pushk --message="$(MESSAGE)" --use-temp-checkout gitsync
