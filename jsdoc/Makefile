include ../make/bazel.mk
include ../kube/kube.mk

.PHONY: build docs release push
build:
	$(BAZEL) build //jsdoc/go/...

release: build
	# Populate infra-sk directory.
	cp ../infra-sk/dist/* infra-sk/

	# Populate the docs/ directory.
	../node_modules/.bin/jsdoc -c jsdoc.config.js -d docs
	chmod 755 -R docs

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
	$(BAZEL) run //jsdoc:push_jsdoc_container

	# Cleanup.
	git clean -fxq docs/* elements-sk/* infra-sk/*

push: pushk release
	pushk --use-temp-checkout jsdoc

# Set DOCKER_IMAGE to the image you want to run locally.
run-local:
	docker run -p 8000:8000 -ti $(DOCKER_IMAGE)
