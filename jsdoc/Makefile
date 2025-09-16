include ../make/bazel.mk

push:
	$(BAZEL) run //jsdoc:jsdoc_push

run-local:
	$(BAZEL) run //jsdoc:jsdoc_local
	docker run -p 8000:8000 -ti jsdoc:latest
