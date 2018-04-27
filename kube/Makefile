build:

	CGO_ENABLED=0 GOOS=linux go install -a -installsuffix cgo ./go/...
	CGO_ENABLED=0 GOOS=linux go install -a -installsuffix cgo github.com/jimmidyson/configmap-reload

push_configmap_reload: build
	./build_configmap_reload_release
	pushk configmap-reload

push_iap_proxy: build
	./build_iap_proxy_release
	pushk iap_proxy
