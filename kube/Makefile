.PHONY: build test pushk release_configmap_reload release_expand_and_reload release_iap_proxy push release_continuous_deploy

build:
	go install  ./go/...

test:
	go test ./go/...

pushk:
	go install ./go/pushk

release_configmap_reload:
	CGO_ENABLED=0 GOOS=linux go install -a github.com/jimmidyson/configmap-reload
	./build_configmap_reload_release

release_expand_and_reload:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/expand-and-reload
	./build_expand_and_reload_release

release_iap_proxy:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/iap-proxy
	./build_iap_proxy_release

release_continuous_deploy:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/continuous-deploy ./go/pushk
	./build_continuous_deploy_release

push: pushk
	pushk iap-proxy configmap-reload expand-and-reload continuous-deploy

