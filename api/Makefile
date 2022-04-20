include ../make/bazel.mk

build:
	$(BAZEL) build go/...

.PHONY: docs
docs:
	cd $(SKIA)/tools/doxygen; doxygen ./Doxyfile

release: docs
	cp -r /tmp/doxygen/html/* docs/
	$(BAZEL) run //api:push_api_container
	# Cleanup the doc directory (silently).
	git clean -f -d -q docs/

# Used by the autodeployment framework:
# https://skia.googlesource.com/buildbot/+doc/main/docker_pushes_watcher/README.md
release-api-ci:
	# COPY_FROM_DIR and STABLE_DOCKER_TAG env variables are expected to be
	# set by the auto deploy mechanism.
	cp -r $(COPY_FROM_DIR)/* docs/
	$(BAZEL) run //api:push_api_container --workspace_status_command=bazel/override_container_label.sh
	# Cleanup the doc directory (silently).
	git clean -f -d -q docs/

push: release
	$(BAZEL) run //kube/go/pushk -- --use-temp-checkout api
