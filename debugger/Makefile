.PHONY: build
build:
	go install -v ./go/skdebug

.PHONY: image
image:
	CGO_ENABLED=0 GOOS=linux go install -installsuffix cgo ./go/skdebug
	SKIP_UPLOAD=1 ./build_docker_release

.PHONY: release
release:
	CGO_ENABLED=0 GOOS=linux go install -a -installsuffix cgo ./go/skdebug
	./build_docker_release

.PHONY: testgo
testgo:
	go test -v ./...
