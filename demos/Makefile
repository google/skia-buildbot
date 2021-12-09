include ../make/bazel.mk

release:
	$(BAZEL) run //demos:push_demos_container

push: release
	pushk demoserver
