default:
	go install -v ./go/logmetrics

.PHONY: push
push: default
	./build_release "`git log -n1 --format=%s`"
	go install -v ../push/go/pushcli
	pushcli logmetrics skia-monitoring

testgo:
	go test -v ./...

