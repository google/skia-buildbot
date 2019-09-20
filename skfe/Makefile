default:
	go install -v ./go/certpoller

loadtest:
	go install -v ./go/loadtest

release: default
	./build_release "`git log -n1 --format=%s`"

# Build and push targets for nginx-skia-org running in skia-public.
k8s_release:
	./build_services_conf
	./build_nginx_skia_org_release "`git log -n1 --format=%s`"

k8s_push: k8s_release
	pushk nginx-skia-org

k8s_run:
	# You can test the config locally by running this target and observing that
	# nginx doesn't fail.
	docker run nginx-skia-org:latest

envoy-release:
	./build_envoy_json
	./build_nginx_skia_org_release

envoy-validate:
	docker run -ti --entrypoint=/usr/local/bin/envoy -v `pwd`/computed.json:/tmp/envoy/computed.json envoyproxy/envoy-dev:latest --mode validate --config-path /tmp/envoy/computed.json

envoy-run-local:
	docker run -ti -p 8000:8000 nginx-skia-org:latest --config-path /etc/envoy/computed.json

