include ../make/bazel.mk
include ../kube/kube.mk

build:
	$(BAZEL) build go/k8s-checker/...

release:
	$(BAZEL) run //k8s-checker:push_k8s_checker_container

push: pushk release
	pushk --use-temp-checkout k8s-checker

push_public: release
	pushk --only-cluster skia-public k8s-checker

push_corp: release
	pushk --only-cluster skia-corp k8s-checker
