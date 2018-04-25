push_prom_config:
	kubectl create configmap prometheus-server-conf --from-file=prometheus.yml -o yaml --dry-run | kubectl replace -f -

push_alertmanager_config:
	kubectl create configmap alertmanager-conf --from-file=alertmanager-config.yml -o yaml --dry-run | kubectl replace -f -

