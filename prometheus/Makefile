build:
	go install -v ./go/webhook_email_proxy

prom:
	go get -u github.com/prometheus/prometheus/cmd/...

testgo:
	go test -v ./...

push:
	promtool check-config sys/prometheus.yml
	./build_config_release "`git log -n1 --format=%s`"
	go install -v ../push/go/pushcli
	pushcli prometheus-config skia-prom

push_binaries: build prom alertmanager
	promtool check-config sys/prometheus.yml
	./build_release "`git log -n1 --format=%s`"
	go install -v ../push/go/pushcli
	pushcli prometheus skia-prom

alertmanager:
	-mkdir -p tmp
	curl -L https://github.com/prometheus/alertmanager/releases/download/v0.5.1/alertmanager-0.5.1.linux-amd64.tar.gz  | tar zxC ./tmp --overwrite -f -
	cp ./tmp/alertmanager-0.5.1.linux-amd64/alertmanager ./tmp/alertmanager

.PHONY: build push alertmanager
