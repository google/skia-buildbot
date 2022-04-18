include ../make/bazel.mk

serve:
	@echo "Bazel doesn't have a serve-all option."
	@echo "For the module under development, try running something like:"
	@echo "bazel run //new_element/modules/example-control-sk:demo_page_server"

test-js:
	$(BAZEL) test //new_element/modules/...

build:
	$(BAZEL) build //new_element/...
