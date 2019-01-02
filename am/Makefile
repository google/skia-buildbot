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
	pushk --cluster=skia-public alert-manager alert-to-pubsub
	pushk --cluster=skia-corp alert-to-pubsub


push_am: release
	pushk --cluster=skia-public alert-manager

serve: package-lock.json
	npx webpack-dev-server --mode=development --watch-poll

testgo:
	go test ./go/...

test:
	# Run the generated tests just once under Xvfb.
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

testci:
	rm -rf node_modules
	npm install
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

watch: package-lock.json
	npx webpack --mode=development --watch

package-lock.json: package.json
	npm install
