
.PHONY: validate
validate: ./tmp/promtool absent
	promk-absent --input=prometheus/alerts_alert_to_pubsub.yml --output=prometheus/absent_alerts_alert_to_pubsub.yml
	promk-absent --input=prometheus/alerts_corp.yml --output=prometheus/absent_alerts_corp.yml
	promk-absent --input=prometheus/alerts_general.yml --output=prometheus/absent_alerts_general.yml
	promk-absent --input=prometheus/alerts_public.yml --output=prometheus/absent_alerts_public.yml
	-./tmp/promtool check config ./prometheus/prometheus-public.yml
	-./tmp/promtool check config ./prometheus/prometheus-corp.yml
	./tmp/promtool check rules ./prometheus/alerts_*.yml
	./tmp/promtool check rules ./prometheus/absent_*.yml
	# Please check output above for FAILED. If the only error is "FAILED: error checking bearer
	# token file..." then you should be good.

.PHONY: absent
absent:
	go install ./go/promk-absent

.PHONY: testgo
testgo:
	go test ./go/...

./tmp/promtool:
	-mkdir -p tmp
	curl -L https://github.com/prometheus/prometheus/releases/download/v2.2.1/prometheus-2.2.1.linux-amd64.tar.gz | tar zxC ./tmp --overwrite -f -
	cp ./tmp/prometheus-2.2.1.linux-amd64/promtool ./tmp/promtool

.PHONY: push_config_public
push_config_public: validate skia-public
	-rm -rf ./tmpconf
	mkdir ./tmpconf
	cp prometheus/prometheus-public.yml ./tmpconf/prometheus.yml
	cp prometheus/alerts_*.yml ./tmpconf
	cp prometheus/absent_alerts_*.yml ./tmpconf
	# Need to use replace so that the configmap gets updated. Change "replace"
	# to "create" if this is the first time the configmap has been uploaded.
	kubectl create configmap prometheus-server-conf --from-file=./tmpconf -o yaml --dry-run | kubectl replace -f -

.PHONY: backup-to-gcs
backup-to-gcs: skia-public
	CGO_ENABLED=0 GOOS=linux go install -a ./go/backup-to-gcs
	./build_backup_to_gcs_release "`git log -n1 --format=%s`"
	pushk backup-to-gcs

.PHONY: push_config_corp
push_config_corp: validate skia-corp
	# Need to use replace so that the configmap gets updated. Change "replace"
	# to "create" if this is the first time the configmap has been uploaded.
	-rm -rf ./tmpconf
	mkdir ./tmpconf
	cp prometheus/prometheus-corp.yml ./tmpconf/prometheus.yml
	cp prometheus/alerts_*.yml ./tmpconf
	cp prometheus/absent_alerts_*.yml ./tmpconf
	kubectl create configmap prometheus-server-conf --from-file=./tmpconf -o yaml --dry-run | kubectl replace -f -

.PHONY: push_config
push_config: push_config_public push_config_corp

include ../make/clusters.mk
