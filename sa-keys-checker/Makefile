include ../make/bazel.mk

build:
	$(BAZEL) build go/sa-keys-checker/...

testgo:
	go test ./go/...

release:
	$(BAZEL) run //sa-keys-checker:push_sa_keys_checker_container

push: pushk release
	$(BAZEL) run //kube/pushk -- --use-temp-checkout sa-keys-checker
