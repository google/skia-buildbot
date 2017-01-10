build:
	go install -v ./go/corpproxy

push: build
	./build_release "`git log -n1 --format=%s`"
	go install -v ../push/go/pushcli
	pushcli corpproxy skia-proxy

.PHONY: build push
