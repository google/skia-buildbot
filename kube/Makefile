.PHONY: build test pushk release_configmap_reload release_iap_proxy push

build:
	go install  ./go/...

test:
	go test ./go/...

pushk:
	go install ./go/pushk

release_configmap_reload:
	CGO_ENABLED=0 GOOS=linux go install -a -installsuffix cgo github.com/jimmidyson/configmap-reload
	./build_configmap_reload_release

release_iap_proxy:
	CGO_ENABLED=0 GOOS=linux go install -a -installsuffix cgo ./go/iap_proxy
	./build_iap_proxy_release

push: pushk
	pushk iap_proxy configmap-reload

