include ../make/bazel.mk

build:
	$(BAZEL) build go/k8s-deployer/...

release:
	$(BAZEL) run //k8s-deployer:push_k8s_deployer_container
