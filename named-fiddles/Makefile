include ../make/bazel.mk

build:
	$(BAZEL) build --config=mayberemote //named-fiddles/go/...

release: build
	$(BAZEL) run --config=mayberemote //named-fiddles:push_named_fiddles_container

push: pushk release
	$(BAZEL) run --config=mayberemote //kube/go/pushk -- --use-temp-checkout named-fiddles

testci:
	go test ./go/...
