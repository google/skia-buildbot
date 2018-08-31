.PHONY: build test pushk release_configmap_reload release_auth_push release_continuous_deploy

build:
	go install  ./go/...

test:
	go test ./go/...

pushk:
	go install ./go/pushk

release_configmap_reload:
	CGO_ENABLED=0 GOOS=linux go install -a github.com/jimmidyson/configmap-reload
	./build_configmap_reload_release

release_auth_proxy:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/auth-proxy
	./build_auth_proxy_release

release_continuous_deploy:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/continuous-deploy ./go/pushk
	./build_continuous_deploy_release

push: pushk
	pushk --cluster=skia-public auth-proxy configmap-reload continuous-deploy
	pushk --cluster=skia-corp configmap-reload
