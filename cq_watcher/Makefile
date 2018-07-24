default:
	go install -v ./go/...

release: default
	CGO_ENABLED=0 GOOS=linux go install -a -installsuffix cgo ./go/cq_watcher
	./build_docker_release

push: default
	pushk cq_watcher
