include ../make/bazel.mk

export VERSION:=$(shell ../bash/release_tag.sh)

.PHONY: validate
validate: ./tmp/promtool absent
	promk-absent --input=prometheus/alerts_alert_to_pubsub.yml --output=prometheus/absent_alerts_alert_to_pubsub.yml
	promk-absent --input=prometheus/alerts_corp.yml --output=prometheus/absent_alerts_corp.yml
	promk-absent --input=prometheus/alerts_general.yml --output=prometheus/absent_alerts_general.yml
	promk-absent --input=prometheus/alerts_public.yml --output=prometheus/absent_alerts_public.yml
	promk-absent --input=prometheus/alerts_thanos.yml --output=prometheus/absent_alerts_thanos.yml
	promk-absent --input=prometheus/alerts_perf_cockroachdb.yml --output=prometheus/absent_alerts_perf_cockroachdb.yml
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

.PHONY: gen_configmaps
gen_configmaps: validate
	-rm -rf ./tmpconf
	mkdir ./tmpconf
	./gen-configmaps.sh ./tmpconf

.PHONY: push_config_thanos
push_config_thanos: gen_configmaps skia-public
	# Need to use replace so that the configmap gets updated. Change "replace"
	# to "create" if this is the first time the configmap has been uploaded.
	kubectl replace -f ./tmpconf/skia-public/thanos-rules.yml

.PHONY: push_config_public
push_config_public: gen_configmaps skia-public
	# Need to use replace so that the configmap gets updated. Change "replace"
	# to "create" if this is the first time the configmap has been uploaded.
	kubectl replace -f ./tmpconf/skia-public/prometheus-server-conf.yml

.PHONY: backup-to-gcs
backup-to-gcs:
	$(BAZEL) run //promk:push_backup_to_gcs_container
	$(BAZEL) run //kube/go/pushk -- backup-to-gcs

.PHONY: push_config_corp
push_config_corp: gen_configmaps skia-corp
	# Need to use replace so that the configmap gets updated. Change "replace"
	# to "create" if this is the first time the configmap has been uploaded.
	kubectl replace -f ./tmpconf/skia-public/prometheus-server-conf.yml

.PHONY: push_config
push_config: push_config_public push_config_corp

release_thanos:
	./images/thanos/release

release_thanos_bouce:
	./images/thanos-bounce/release

push_thanos:
	$(BAZEL) run //kube/go/pushk -- thanos

push_thanos_bounce:
	$(BAZEL) run //kube/go/pushk -- thanos-bounce

.PHONY: update_alerts
update_alerts: push_config_thanos

# Builds kube-state-metrics and pushes to our registry using our local tag
# naming convention.
release_kube-state-metrics:
	git clone -b v2.4.2 https://github.com/kubernetes/kube-state-metrics.git
	cd kube-state-metrics && REGISTRY=gcr.io/skia-public TAG=$(VERSION) make container
	rm -rf kube-state-metrics
	docker push gcr.io/skia-public/kube-state-metrics-amd64:$(VERSION)

include ../make/clusters.mk
