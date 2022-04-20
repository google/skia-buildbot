include ../make/bazel.mk

build:
	$(BAZEL) build go/dotserver/...

run-local-instance: build
	../_bazel_bin/dot/go/dotserver/dotserver_/dotserver --local

release: build
	$(BAZEL) run //dot:push_dotserver_container

push: release
	$(BAZEL) run //kube/go/pushk -- --use-temp-checkout dotserver

test:
	$(BAZEL) test go/...
