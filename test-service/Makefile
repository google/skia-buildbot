include ../make/bazel.mk

.PHONY: build
build:
	$(BAZEL) build --config=mayberemote //test-service/go/...

.PHONY: release
release: build
	$(BAZEL) run --config=mayberemote //test-service:push_test_service_container

