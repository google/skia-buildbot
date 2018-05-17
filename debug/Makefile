.PHONY: build
build:
	go install -v ./go/debug

.PHONY: image
image:
	CGO_ENABLED=0 GOOS=linux go install -installsuffix cgo ./go/debug
	SKIP_UPLOAD=1 ./build_docker_release

.PHONY: release
release:
	CGO_ENABLED=0 GOOS=linux go install -a -installsuffix cgo ./go/debug
	./build_docker_release

.PHONY: push
push:
	go install ../kube/go/pushk
	pushk debug

.PHONY: testgo
testgo:
	go test -v ./...

.PHONY: run
run:
	 docker run -p8000:8000 -ti debug

.PHONY: tag_skiaserve
tag_skiaserve:
	./tag-latest-skiaserve.sh
