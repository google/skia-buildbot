.PHONY: g3_compile
g3_compile:
	go install -v ./go/...

.PHONY: release
release:
	GOOS=linux go install -a ./go/...
	./build_docker_release

.PHONY: push
push: skia-corp
	pushk --cluster=skia-corp g3_compile

include ../make/clusters.mk
