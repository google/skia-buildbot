include ../make/bazel.mk

.PHONY: build-images
build-images:
	$(BAZEL) build //cd/go/build-images:build-images

.PHONY: release
release: build-images
	./build_release