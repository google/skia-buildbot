include ../make/bazel.mk
include ../kube/kube.mk

build:
	$(BAZEL) build //named-fiddles/go/...

release: build
	$(BAZEL) run //named-fiddles:push_named_fiddles_container

push: pushk release
	pushk --use-temp-checkout named-fiddles

testci:
	go test ./go/...
