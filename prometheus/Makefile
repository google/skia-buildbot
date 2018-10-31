build:
	go install -v ./go/webhook_proxy
	go install -v ./go/proxy_with_auth
	go install -v ./go/absent
	absent

prom:
	-mkdir -p tmp
	curl -L https://github.com/prometheus/prometheus/releases/download/v1.8.0/prometheus-1.8.0.linux-amd64.tar.gz | tar zxC ./tmp --overwrite -f -
	cp ./tmp/prometheus-1.8.0.linux-amd64/prometheus ./tmp/prometheus
	cp ./tmp/prometheus-1.8.0.linux-amd64/promtool ./tmp/promtool

testgo:
	go test -v ./...

promtool: prom
	cp ./tmp/prometheus-1.8.0.linux-amd64/promtool ./tmp/promtool

validate: build promtool
	absent
	./tmp/promtool check-config sys/prometheus.yml

push: validate
	./build_config_release "`git log -n1 --format=%s`"
	go install -v ../push/go/pushcli
	pushcli prometheus-config skia-prom

push_binaries: build prom pushgateway validate
	./build_release "`git log -n1 --format=%s`"
	go install -v ../push/go/pushcli
	pushcli prometheus skia-prom

push_skolo: build prom
	./tmp/promtool check-config sys/jumphost-internal-01/prometheus.yml
	./tmp/promtool check-config sys/jumphost-rpi-01/prometheus.yml
	./tmp/promtool check-config sys/jumphost-linux-01/prometheus.yml
	./tmp/promtool check-config sys/jumphost-win-02/prometheus.yml
	./tmp/promtool check-config sys/jumphost-win-03/prometheus.yml
	CONFIG=internal-01 ./build_skolo_release "`git log -n1 --format=%s`"
	CONFIG=rpi-01      ./build_skolo_release "`git log -n1 --format=%s`"
	CONFIG=linux-01    ./build_skolo_release "`git log -n1 --format=%s`"
	CONFIG=win-02      ./build_skolo_release "`git log -n1 --format=%s`"
	CONFIG=win-03      ./build_skolo_release "`git log -n1 --format=%s`"
	go install -v ../push/go/pushcli
	pushcli prometheus-jumphost-internal-01 jumphost-internal-01
	pushcli prometheus-jumphost-rpi-01      jumphost-rpi-01
	pushcli prometheus-jumphost-linux-01    jumphost-linux-01
	pushcli prometheus-jumphost-win-02      jumphost-win-02
	pushcli prometheus-jumphost-win-03      jumphost-win-03

push_graphite_exporter: graphite_exporter
	./build_graphite_exporter_release "`git log -n1 --format=%s`"
	go install -v ../push/go/pushcli
	pushcli graphite-exporter skia-monitoring

pushgateway:
	-mkdir -p tmp
	curl -L https://github.com/prometheus/pushgateway/releases/download/v0.3.1/pushgateway-0.3.1.linux-amd64.tar.gz  | tar zxC ./tmp --overwrite -f -
	cp ./tmp/pushgateway-0.3.1.linux-amd64/pushgateway ./tmp/pushgateway

graphite_exporter:
	-mkdir -p tmp
	curl -L https://github.com/prometheus/graphite_exporter/releases/download/v0.2.0/graphite_exporter-0.2.0.linux-amd64.tar.gz  | tar zxC ./tmp --overwrite -f -
	cp ./tmp/graphite_exporter-0.2.0.linux-amd64/graphite_exporter ./tmp/graphite_exporter

.PHONY: build push push_binaries graphite_exporter validate
