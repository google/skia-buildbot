.PHONY: build
build:
	go install -v ./go/skdebug

.PHONY: release
release: 
	CGO_ENABLED=0 GOOS=linux go install -a -installsuffix cgo ./go/skdebug
	SKIP_UPLOAD=1 ./build_docker_release

.PHONY: testgo
testgo:
	go test -v ./...

run_server_local: build
	skdebugger --port=localhost:9000 --local --alsologtostderr

include ../webtools/webtools.mk
