include ../make/bazel.mk
include ../kube/kube.mk

serve:
	@echo "Bazel doesn't have a serve-all option."                          
	@echo "For the module under development, try running something like:"   
	@echo "bazel run //infra-sk/modules/app-sk:demo_page_server"

test-js:                                                                         
	$(BAZEL) test modules/...

build:
	$(BAZEL) build //infra-sk/...
