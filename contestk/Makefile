.PHONY: build release push
build:
	go install  ./go/...

release:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/contest
	./build_docker_release

push: release
	pushk contest