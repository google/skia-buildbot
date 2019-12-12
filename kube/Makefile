.PHONY: build test pushk release_configmap_reload release_auth_push release_continuous_deploy

build:
	go install  ./go/...

test:
	go test ./go/...

release_configmap_reload:
	CGO_ENABLED=0 GOOS=linux go install -a github.com/jimmidyson/configmap-reload
	./build_configmap_reload_release

release_auth_proxy:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/auth-proxy
	./build_auth_proxy_release

release_continuous_deploy:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/continuous-deploy ./go/pushk
	./build_continuous_deploy_release

release_basealpine:
	docker pull alpine:3.8
	./build_basealpine_release

push_reposync:
	go install ./go/repo-sync
	./build_reposync_release
	pushk repo-sync

push: release_auth_proxy release_continuous_deploy release_configmap_reload
	pushk auth-proxy configmap-reload continuous-deploy

pushk_authproxy: release_auth_proxy
	pushk auth-proxy

include kube.mk
