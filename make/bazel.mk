# Helper rules for Bazel.

# BAZEL defines which executable to run.
#
# If BAZEL isn't defined then try to define it as bazelisk (if the executable exists).
# Otherwise, define it as bazel.
ifeq ($(BAZEL),)
	ifneq ($(strip $(shell which bazelisk)),)
		BAZEL := bazelisk
	else
		BAZEL := bazel
	endif
endif