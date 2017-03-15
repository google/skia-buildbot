build:
	go install -v ./go/webhook_proxy
	go install -v ./go/proxy_with_auth
	go install -v ./go/absent
	absent

prom:
	go get -u github.com/prometheus/prometheus/cmd/...

testgo:
	go test -v ./...

validate: build
	absent
	promtool check-config sys/prometheus.yml
	promtool check-config sys/prometheus-skolo.yml

push: validate
	./build_config_release "`git log -n1 --format=%s`"
	go install -v ../push/go/pushcli
	pushcli prometheus-config skia-prom

push_binaries: build prom alertmanager pushgateway validate
	./build_release "`git log -n1 --format=%s`"
	go install -v ../push/go/pushcli
	pushcli prometheus skia-prom

push_skolo: build prom
	promtool check-config sys/prometheus-skolo.yml
	./build_skolo_release "`git log -n1 --format=%s`"
	go install -v ../push/go/pushcli
	pushcli prometheus-skolo skia-jumphost

push_graphite_exporter: graphite_exporter
	./build_graphite_exporter_release "`git log -n1 --format=%s`"
	go install -v ../push/go/pushcli
	pushcli graphite-exporter skia-monitoring

alertmanager:
	-mkdir -p tmp
	curl -L https://github.com/prometheus/alertmanager/releases/download/v0.5.1/alertmanager-0.5.1.linux-amd64.tar.gz  | tar zxC ./tmp --overwrite -f -
	cp ./tmp/alertmanager-0.5.1.linux-amd64/alertmanager ./tmp/alertmanager

pushgateway:
	-mkdir -p tmp
	curl -L https://github.com/prometheus/pushgateway/releases/download/v0.3.1/pushgateway-0.3.1.linux-amd64.tar.gz  | tar zxC ./tmp --overwrite -f -
	cp ./tmp/pushgateway-0.3.1.linux-amd64/pushgateway ./tmp/pushgateway

graphite_exporter:
	go get -u github.com/prometheus/graphite_exporter
	cd ${GOPATH}/src/github.com/prometheus/graphite_exporter; make


.PHONY: build push push_binaries alertmanager graphite_exporter validate
