datahopper:
	go install -v ./go/datahopper

all: datahopper

testgo:
	go test ./...

release: datahopper
	./build_release "`git log -n1 --format=%s`"

datahopper_kube:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/datahopper

release_kube: datahopper_kube
	./build_docker_release

.PHONY: datahopper release testgo
