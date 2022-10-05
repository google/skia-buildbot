include ../make/bazel.mk

build:
	$(BAZEL) build --config=mayberemote go/k8s-checker/...

release:
	$(BAZEL) run --config=mayberemote //k8s-checker:push_k8s_checker_container

push: release
	$(BAZEL) run --config=mayberemote //kube/go/pushk -- --use-temp-checkout k8s-checker

push_public: release
	$(BAZEL) run --config=mayberemote //kube/go/pushk -- --only-cluster skia-public k8s-checker

push_corp: release
	$(BAZEL) run --config=mayberemote //kube/go/pushk -- --only-cluster skia-corp k8s-checker
