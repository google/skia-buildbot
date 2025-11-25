include ../../make/bazel.mk

.PHONY: test
test-regressions:
	$(BAZEL) test //perf/smoke_tests:regression_page_nodejs_test

test-regressions-crd:
	DISPLAY=:20 $(BAZEL) test //perf/smoke_tests:regression_page_nodejs_test \
	--test_env=DEBUG_VIA_CRD=true --test_output=streamed
