.PHONY: validate
validate:
	cat prometheus.yml | docker run --entrypoint=/bin/promtool -i prom/prometheus check config /dev/stdin

.PHONY: push_config
push_config: validate
	# Need to use replace so that the configmap gets updated. Change "replace"
	# to "create" if this is the first time the configmap has been uploaded.
	kubectl create configmap prometheus-server-conf --from-file=prometheus.yml -o yaml --dry-run | kubectl replace -f -
	kubectl create configmap alertmanager-conf --from-file=alertmanager.yml -o yaml --dry-run | kubectl replace -f -
