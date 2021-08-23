# Helper rules for Bazel.

# BAZEL defines which executable to run.
#
# If BAZEL isn't defined then define it as bazelisk.
BAZEL := $(or $(BAZEL),bazelisk)