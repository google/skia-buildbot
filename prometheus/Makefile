build:
	go install -v ./go/webhook_proxy
	go install -v ./go/proxy_with_auth
	go install -v ./go/absent
	go install -v ./../misc/./go/sheriff_emails/...
	absent

prom:
	-mkdir -p tmp
	curl -L https://github.com/prometheus/prometheus/releases/download/v1.5.2/prometheus-1.5.2.linux-amd64.tar.gz | tar zxC ./tmp --overwrite -f -
	cp ./tmp/prometheus-1.5.2.linux-amd64/prometheus ./tmp/prometheus

testgo:
	go test -v ./...

promtool:
	go get github.com/prometheus/prometheus/cmd/promtool

validate: build promtool
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

push_sheriff_emails: build
	./build_config_release "`git log -n1 --format=%s`"
	go install -v ../push/go/pushcli
	pushcli sheriff-emails

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
	-mkdir -p tmp
	curl -L https://github.com/prometheus/graphite_exporter/releases/download/v0.2.0/graphite_exporter-0.2.0.linux-amd64.tar.gz  | tar zxC ./tmp --overwrite -f -
	cp ./tmp/graphite_exporter-0.2.0.linux-amd64/graphite_exporter ./tmp/graphite_exporter

.PHONY: build push push_binaries alertmanager graphite_exporter validate
