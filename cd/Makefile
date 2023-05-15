include ../make/bazel.mk

.PHONY: build-images
build-images:
	$(BAZEL) build --config=mayberemote //cd/go/build-images:build-images

.PHONY: release
release: build-images
	./build_release

.PHONY: docs
docs:
	$(BAZEL) run --config=mayberemote //cd/cmd/stagemanager:stagemanager -- markdown > STAGEMANAGER.md