include ../make/bazel.mk
include ../kube/kube.mk

.PHONY: build release push
build:
	${BAZEL} build //contestk/go/...

release: build
	${BAZEL} build //contestk:push_contest_container

push: pushk release
	pushk --use-temp-checkout contest
