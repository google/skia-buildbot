include ../make/bazel.mk

build-config:
	go install ./go/merge_envoy
	go install ./go/update_probers
	update_probers
	../kube/attach.sh skia-public  ./build_envoy_json

release: build-config
	$(BAZEL) run //skfe:push_skfe_container

validate: build-config
	docker run -ti --entrypoint=/usr/local/bin/envoy -v `pwd`/computed.json:/tmp/envoy/computed.json envoyproxy/envoy:v1.16.1 --mode validate --config-path /tmp/envoy/computed.json

push: pushk release validate
	$(BAZEL) run //kube/go/pushk -- --use-temp-checkout envoy-skia-org

# Set DOCKER_IMAGE to the image you want to run locally.
run-local:
	docker run -ti -p 8000:8000 $(DOCKER_IMAGE) --config-path /etc/envoy/computed.json

test:
	$(BAZEL) test //skfe/go/dns:dns_test
