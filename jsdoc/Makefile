include ../make/bazel.mk

release:
	$(BAZEL) run --config=mayberemote //jsdoc:push_jsdoc_container

# Set DOCKER_IMAGE to the image you want to run locally.
run-local:
	docker run -p 8000:8000 -ti $(DOCKER_IMAGE)
