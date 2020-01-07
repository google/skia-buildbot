.PHONY: build test pushk release push

build:
	go install  ./go/...

test:
	go test ./go/...

release: build
	CGO_ENABLED=0 GOOS=linux go install -a ./go/auto-deploy ./../kube/go/pushk
	./build_docker_release

push: release
	pushk auto-deploy
