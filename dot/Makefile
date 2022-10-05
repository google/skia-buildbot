include ../make/bazel.mk

build:
	$(BAZEL) build --config=mayberemote go/dotserver/...

run-local-instance: build
	../_bazel_bin/dot/go/dotserver/dotserver_/dotserver --local

release: build
	$(BAZEL) run --config=mayberemote //dot:push_dotserver_container

push: release
	$(BAZEL) run --config=mayberemote //kube/go/pushk -- --use-temp-checkout dotserver

test:
	$(BAZEL) test --config=mayberemote go/...
