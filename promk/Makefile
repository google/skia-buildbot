.PHONY: validate push_config
validate: ./tmp/promtool
	# If the only error is "FAILED: error checking bearer token file..." then you should be good.
	-./tmp/promtool check config ./prometheus/prometheus-public.yml
	-./tmp/promtool check config ./prometheus/prometheus-corp.yml
	./tmp/promtool check rules ./prometheus/alerts_*.yml

./tmp/promtool:
	-mkdir -p tmp
	curl -L https://github.com/prometheus/prometheus/releases/download/v2.2.1/prometheus-2.2.1.linux-amd64.tar.gz | tar zxC ./tmp --overwrite -f -
	cp ./tmp/prometheus-2.2.1.linux-amd64/promtool ./tmp/promtool

push_config: validate skia-public
	kubectl create configmap grafana-conf --from-file=./grafana -o yaml --dry-run | kubectl replace -f -
	-rm -rf ./tmpconf
	mkdir ./tmpconf
	cp prometheus/prometheus-public.yml ./tmpconf/prometheus.yml
	cp prometheus/alerts_*.yml ./tmpconf
	# Need to use replace so that the configmap gets updated. Change "replace"
	# to "create" if this is the first time the configmap has been uploaded.
	kubectl create configmap prometheus-server-conf --from-file=./tmpconf -o yaml --dry-run | kubectl replace -f -

push_config_corp: validate skia-corp
	# Need to use replace so that the configmap gets updated. Change "replace"
	# to "create" if this is the first time the configmap has been uploaded.
	-rm -rf ./tmpconf
	mkdir ./tmpconf
	cp prometheus/prometheus-corp.yml ./tmpconf/prometheus.yml
	cp prometheus/alerts_*.yml ./tmpconf
	kubectl create configmap prometheus-server-conf --from-file=./tmpconf -o yaml --dry-run | kubectl replace -f -

include ../make/clusters.mk
