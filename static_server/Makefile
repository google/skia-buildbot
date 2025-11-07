include ../make/bazel.mk

.PHONY: release
release:
	$(BAZEL) run --config=mayberemote //static_server:push_static_server
