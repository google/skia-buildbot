include ../make/bazel.mk

.PHONY: build docs release push
build:
	$(BAZEL) build --config=mayberemote //jsdoc/go/...

release: build
	npm ci
	# Populate the docs/ directory.
	../node_modules/.bin/jsdoc -c jsdoc.config.js -d docs
	chmod 755 -R docs

	# If the elements-sk or docs doesn't seem to be updating try running `bazelisk clean --expunge`.
	# All of this needs to eventually be moved to Bazel.

	# Populate the elements-sk/ directory.
	PWD=$(shell pwd);
	cd /tmp/; \
	echo `pwd`; \
	rm -rf elements-sk; \
	git clone https://github.com/google/elements-sk.git; \
	cd /tmp/elements-sk; \
	echo `pwd`; \
	npm ci; \
	make; \
	cd $(PWD); \
	echo `pwd`; \
	cp /tmp/elements-sk/dist/* elements-sk/;

	# Create the docker container.
	$(BAZEL) run --config=mayberemote //jsdoc:push_jsdoc_container

	# Cleanup.
	git clean -fxq docs/* elements-sk/*

push: release
	$(BAZEL) run --config=mayberemote //kube/go/pushk -- --use-temp-checkout jsdoc

# Set DOCKER_IMAGE to the image you want to run locally.
run-local:
	docker run -p 8000:8000 -ti $(DOCKER_IMAGE)
