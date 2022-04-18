include ../make/bazel.mk

build:
	$(BAZEL) build //named-fiddles/go/...

release: build
	$(BAZEL) run //named-fiddles:push_named_fiddles_container

push: pushk release
	$(BAZEL) run //kube/pushk -- --use-temp-checkout named-fiddles

testci:
	go test ./go/...
