.PHONY: build test pushk release_configmap_reload release_expand_and_reload release_iap_proxy push

build:
	go install  ./go/...

test:
	go test ./go/...

pushk:
	go install ./go/pushk

release_configmap_reload:
	CGO_ENABLED=0 GOOS=linux go install -a -installsuffix cgo github.com/jimmidyson/configmap-reload
	./build_configmap_reload_release

release_expand_and_reload:
	CGO_ENABLED=0 GOOS=linux go install -a -installsuffix cgo ./go/expand-and-reload
	./build_expand_and_reload_release

release_iap_proxy:
	CGO_ENABLED=0 GOOS=linux go install -a -installsuffix cgo ./go/iap-proxy
	./build_iap_proxy_release

push: pushk
	pushk iap-proxy configmap_reload expand-and-reload

