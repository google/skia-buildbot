build: core
	npx webpack

serve:
	npx webpack-dev-server --watch

core:
	yarn
	go install -v ./go/push
	go install -v ./go/pushcli
	go install -v ./go/build_skiapush_json5
	build_skiapush_json5

release: core
	NODE_ENV=production npx webpack

watch: core
	npx webpack --watch

push: release
	./build_pushd_release "`git log -n1 --format=%s`"
	pushcli pushd skia-push

.PHONY: build core release push

