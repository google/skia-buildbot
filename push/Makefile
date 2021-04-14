build: core npm-ci
	npx webpack --mode=development

serve: npm-ci
	npx webpack-dev-server --watch --mode=development

core: npm-ci
	go install -v ./go/push
	go install -v ./go/pushcli
	go install -v ./go/build_skiapush_json5
	build_skiapush_json5

testci: test

test: core npm-ci
	# Run the generated tests just once under Xvfb.
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run --no-colors

release: core npm-ci
	CGO_ENABLED=0 GOOS=linux go install -a ./go/push
	npx webpack --mode=production
	./build_push_release "`git log -n1 --format=%s`"

watch: core npm-ci
	npx webpack --watch --mode=development

push: release
	pushk push

docs: npm-ci
	npx jsdoc -c jsdoc.config.js `find modules -name "*.js"`

.PHONY: build serve core test release watch push docs

.PHONY: build-frontend-ci
build-frontend-ci: npm-ci
	npx webpack --mode=production

.PHONY: test-frontend-ci
test-frontend-ci: npm-ci
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

include ../make/npm.mk
