build: core
	npx webpack --mode=development

serve:
	npx webpack-dev-server --watch --mode=development

core: package-lock.json
	go install -v ./go/push
	go install -v ./go/pushcli
	go install -v ./go/build_skiapush_json5
	build_skiapush_json5

test: core
	# Run the generated tests just once under Xvfb.
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run --no-colors

release: core
	npx webpack --mode=production

watch: core
	npx webpack --watch --mode=development

push: release
	./build_pushd_release "`git log -n1 --format=%s`"
	pushcli pushd skia-push

docs:
	npx jsdoc -c jsdoc.config.js `find modules -name "*.js"`

.PHONY: build core release push

package-lock.json: package.json
	npm install
