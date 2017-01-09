build:
	go install -v ./go/corpproxy

push:
	./build_release "`git log -n1 --format=%s`"
	go install -v ../push/go/pushcli
	pushcli prometheus-config skia-prom

.PHONY: build push
