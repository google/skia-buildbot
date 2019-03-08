.PHONY: build docs release push
build:
	go install  ./go/api

docs:
	cd ${SKIA}/tools/doxygen; doxygen ./Doxyfile

release:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/api
	./build_docker_release

push: release skia-public
	pushk api

include ../make/clusters.mk
