build-config:
	go install ./go/merge_envoy
	go install ./go/update_probers
	./build_envoy_json

release: build-config
	./build_envoy_skia_org_release

validate: build-config
	docker run -ti --entrypoint=/usr/local/bin/envoy -v `pwd`/computed.json:/tmp/envoy/computed.json envoyproxy/envoy-dev:latest --mode validate --config-path /tmp/envoy/computed.json

push: release validate
	pushk envoy-skia-org

run-local:
	docker run -ti -p 8000:8000 envoy-skia-org:latest --config-path /etc/envoy/computed.json

legacy-default:
	go install -v ./go/certpoller

legacy-loadtest:
	go install -v ./go/loadtest

legacy-release: legacy-default
	./build_release "`git log -n1 --format=%s`"

