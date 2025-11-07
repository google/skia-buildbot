include ../make/bazel.mk

build:
	$(BAZEL) build --config=mayberemote go/k8s-checker/...

release:
	$(BAZEL) run --config=mayberemote //k8s-checker:push_k8s_checker_container
