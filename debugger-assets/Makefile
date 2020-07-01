# The list of files we want to go into core.js, which is concat'd and
# minified. These files should be either present in the project, brought
# into third_party/bower_components via bower, or in node_modules.
CORE_SOURCE_FILES = node_modules/native-promise-only/npo.js \
					third_party/bower_components/webcomponentsjs/webcomponents-lite.min.js \
					./res/js/debug-canvas.js \
          ../res/js/common.js

BOWER_DIR=third_party/bower_components

VULCANIZE1=true

.PHONY: build
build: package-lock.json elements_html core_js
	go install -v ./go/debugger-assets

# Build debug versions of core.js and elements.html.
.PHONY: debug
debug: clean_webtools debug_core_js debug_elements_html

.PHONY: testgo
testgo:
	go test -v ./...

.PHONY: run_server_local
run_server_local: build
	debugger-assets --port=localhost:9000 --local --logtostderr

.PHONY: run_server_local_wasm
run_server_local_wasm: build
	debugger-assets --port=localhost:9000 --local --logtostderr --v2_at_root --resources_dir .

.PHONY: release
release: build
	docker pull gcr.io/skia-public/skia-wasm-release:prod
	CGO_ENABLED=0 GOOS=linux go install -a ./go/debugger-assets
	./build_docker_release

.PHONY: release_ci
release_ci: build
	# Don't docker pull here, as we are in the CI pipeline and it already has
	# done the pull.
	CGO_ENABLED=0 GOOS=linux go install -a ./go/debugger-assets
	./build_docker_release

.PHONY: push
push: release
	go install ../kube/go/pushk
	pushk debugger-assets

.PHONY: run
run:
	 docker run -p8000:8000 -ti debugger-assets

package-lock.json: package.json
	npm install

include ../webtools/webtools.mk
