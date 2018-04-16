push_config:
	kubectl create configmap prometheus-server-conf --from-file=prometheus.yml -o yaml --dry-run | kubectl replace -f -

