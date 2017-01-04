build:
	go get -u github.com/prometheus/prometheus/cmd/...

.PHONY: push
push: build
	promtool check-config sys/prometheus.yml
	./build_release "`git log -n1 --format=%s`"
	go install -v ../push/go/pushcli
	pushcli prometheus skia-prom
