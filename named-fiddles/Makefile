include ../make/bazel.mk

build:
	$(BAZEL) build --config=mayberemote //named-fiddles/go/...

release: build
	$(BAZEL) run --config=mayberemote //named-fiddles:push_named_fiddles_container

testci:
	go test ./go/...
