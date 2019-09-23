default:
	go install -v ./go/certpoller

loadtest:
	go install -v ./go/loadtest

release: default
	./build_release "`git log -n1 --format=%s`"

envoy-release:
	./build_envoy_json
	./build_envoy_skia_org_release

envoy-validate:
	docker run -ti --entrypoint=/usr/local/bin/envoy -v `pwd`/computed.json:/tmp/envoy/computed.json envoyproxy/envoy-dev:latest --mode validate --config-path /tmp/envoy/computed.json

envoy-push: envoy-release envoy-validate
	pushk envoy-skia-org

envoy-run-local:
	docker run -ti -p 8000:8000 envoy-skia-org:latest --config-path /etc/envoy/computed.json

