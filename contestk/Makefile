include ../make/bazel.mk

.PHONY: build release push
build:
	${BAZEL} build //contestk/go/...

release: build
	${BAZEL} run //contestk:push_contest_container

push: release
	$(BAZEL) run //kube/pushk -- --use-temp-checkout contest
