build:
	go install -v ./go/predict

testgo:
	go test -v ./go/...

.PHONY: push
push: build
	./build_release "`git log -n1 --format=%s`"
	go install -v ../push/go/pushcli
	pushcli predict skia-predict
