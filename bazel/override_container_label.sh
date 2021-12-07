#!/bin/bash -e

# This script is intended to be passed to Bazel using the --workspace_status_command command-line
# flag. Unlike get_workspace_status.sh, this does not set any environment variables, letting
# whatever is in the environment be used.

echo "STABLE_DOCKER_TAG ${STABLE_DOCKER_TAG}"