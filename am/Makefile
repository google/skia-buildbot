default: build package-lock.json
	npx webpack --mode=development

build:
	go install ./go/alert-manager ./go/alert-to-pubsub

release: build package-lock.json
	npx webpack --mode=production
	CGO_ENABLED=0 GOOS=linux go install -a ./go/alert-manager ./go/alert-to-pubsub
	./build_alert_manager_release
	./build_alert_to_pubsub_release

debug:
	npx webpack --mode=development

legacy_release: build package-lock.json
	./build_legacy_alert_to_pubsub_release "`git log -n1 --format=%s`"

push: release
	pushk alert-manager alert-to-pubsub

push_am: release
	pushk alert-manager

serve: package-lock.json
	npx webpack-dev-server --mode=development --watch-poll

testgo:
	go test ./go/...

test:
	# Run the generated tests just once under Xvfb.
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

# Increase Node's heap size to accommodate for ts-node's higher memory usage.
NODE_MEMORY_LIMIT=4096

.PHONY: testpptr
testpptr:
	NODE_OPTIONS="--max-old-space-size=$(NODE_MEMORY_LIMIT)" npx mocha -r ts-node/register ./**/*_puppeteer_test.ts

testci:
	rm -rf node_modules
	npm install
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

watch: package-lock.json
	npx webpack --mode=development --watch

package-lock.json: package.json
	npm install
