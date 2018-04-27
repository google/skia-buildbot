.PHONY: build test pushk push_configmap_reload push_iap_proxy

build:
	go install  ./go/...

test:
	go test ./go/...

pushk:
	go install ./go/pushk

push_configmap_reload: pushk
	CGO_ENABLED=0 GOOS=linux go install -a -installsuffix cgo github.com/jimmidyson/configmap-reload
	./build_configmap_reload_release
	pushk configmap-reload

push_iap_proxy: pushk
	CGO_ENABLED=0 GOOS=linux go install -a -installsuffix cgo ./go/iap_proxy
	./build_iap_proxy_release
	pushk iap_proxy

