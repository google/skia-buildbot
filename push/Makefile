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

docs:
	npx jsdoc -c jsdoc.config.js `find modules -name "*.js"`

test: build
	# Run the generated tests just once under Xvfb.
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

.PHONY: build core release push

