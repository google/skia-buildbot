build:
	go get -u github.com/prometheus/prometheus/cmd/...

.PHONY: push
push: build
	./build_release "`git log -n1 --format=%s`"
	go install -v ../push/go/pushcli
	pushcli prometheus skia-prom
