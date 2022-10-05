include ../make/bazel.mk

build:
	$(BAZEL) build --config=mayberemote go/sa-keys-checker/...

testgo:
	go test ./go/...

release:
	$(BAZEL) run --config=mayberemote //sa-keys-checker:push_sa_keys_checker_container

push: pushk release
	$(BAZEL) run --config=mayberemote //kube/go/pushk -- --use-temp-checkout sa-keys-checker
