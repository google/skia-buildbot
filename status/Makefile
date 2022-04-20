include ../make/bazel.mk

.PHONY: serve
serve:
	@echo "Bazel doesn't have a serve-all option."
	@echo "For the module under development, try running something like:"
	@echo "bazel run //status/modules/autoroller-status-sk:demo_page_server"

testgo:
	$(BAZEL) test //status/go/...

testjs:
	$(BAZEL) test //status/modules/...

build:
	$(BAZEL) build //status/...

release: build
	$(BAZEL) run //status:push_status_container

push: release
	$(BAZEL) run //kube/go/pushk -- --message="$(MESSAGE)" --use-temp-checkout status
