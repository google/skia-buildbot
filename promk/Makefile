
.PHONY: validate
validate: ./tmp/promtool
	./tmp/promtool check config ./prometheus/prometheus.yml

./tmp/promtool:
	-mkdir -p tmp
	curl -L https://github.com/prometheus/prometheus/releases/download/v2.2.1/prometheus-2.2.1.linux-amd64.tar.gz | tar zxC ./tmp --overwrite -f -
	cp ./tmp/prometheus-2.2.1.linux-amd64/promtool ./tmp/promtool

.PHONY: push_config
push: validate
	# Need to use replace so that the configmap gets updated. Change "replace"
	# to "create" if this is the first time the configmap has been uploaded.
	kubectl create configmap prometheus-server-conf --from-file=./prometheus -o yaml --dry-run | kubectl replace -f -
	kubectl create configmap alertmanager-conf --from-file=./alertmanager -o yaml --dry-run | kubectl replace -f -
