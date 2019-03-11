.PHONY: build docs release push
build:
	go install  ./go/api

docs:
	cd ${SKIA}/tools/doxygen; doxygen ./Doxyfile

release:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/api
	./build_docker_release

.PHONY: release_ci
release_ci:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/api
	./build_docker_release_ci

push: release skia-public
	pushk api

include ../make/clusters.mk
