include ../make/bazel.mk

.PHONY: build
build:
	$(BAZEL) build //test-service/go/...

.PHONY: release
release: build
	$(BAZEL) run //test-service:push_test_service_container

