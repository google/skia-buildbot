.PHONY: build
build:
	go install -v ./go/debugger

.PHONY: image
image:
	CGO_ENABLED=0 GOOS=linux go install ./go/debugger
	SKIP_UPLOAD=1 ./build_docker_release

.PHONY: release
release:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/debugger
	./build_docker_release

.PHONY: push
push: release skia-public
	go install ../kube/go/pushk
	pushk debugger

.PHONY: testgo
testgo:
	go test -v ./...

.PHONY: run
run:
	 docker run -p8000:8000 -ti debugger

.PHONY: tag_skiaserve
tag_skiaserve:
	./tag-latest-skiaserve.sh

include ../make/clusters.mk
