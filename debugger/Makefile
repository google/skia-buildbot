.PHONY: build
build:
	go install -v ./go/debugger

.PHONY: image
image:
	CGO_ENABLED=0 GOOS=linux go install -installsuffix cgo ./go/debugger
	SKIP_UPLOAD=1 ./build_docker_release

.PHONY: release
release:
	CGO_ENABLED=0 GOOS=linux go install -a -installsuffix cgo ./go/debugger
	./build_docker_release

.PHONY: push
push:
	go install ../kube/go/pushk
	pushk --apply debugger

.PHONY: testgo
testgo:
	go test -v ./...

.PHONY: run
run:
	 docker run -p8000:8000 -ti debugger

.PHONY: run_against_local_assets_server
run_against_local_assets_server:
	 docker run -ti -p8000:8000  debugger --local --logtostderr --resources_dir=/usr/local/share/debugger --source=http://localhost:9000

.PHONY: tag_skiaserve
tag_skiaserve:
	./tag-latest-skiaserve.sh
