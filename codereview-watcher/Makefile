
default: build

build:
	go install -v ./go/codereview-watcher

release: build
	CGO_ENABLED=0 GOOS=linux go install -a ./go/codereview-watcher
	./images/build_release

push: release
	pushk --use-temp-checkout codereview-watcher

testgo:
	go test ./go/...
