default:
	go install -v ./go/...

release: default
	CGO_ENABLED=0 GOOS=linux go install -a ./go/cq_watcher
	./build_docker_release

push: default skia-public
	pushk cq_watcher

include ../make/clusters.mk
