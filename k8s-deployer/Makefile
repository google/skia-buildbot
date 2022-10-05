include ../make/bazel.mk

build:
	$(BAZEL) build --config=mayberemote go/k8s-deployer/...

release:
	$(BAZEL) run --config=mayberemote //k8s-deployer:push_k8s_deployer_container
