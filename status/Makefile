#Use Vulcanize 1.0
VULCANIZE1=true
# The list of files we want to go into core.js, which is concat'd and
# minified. These files should be either present in the project, brought
# into third_party/bower_compoents via bower, or in node_modules.
CORE_SOURCE_FILES = third_party/bower_components/webcomponentsjs/webcomponents.min.js \
          ../res/js/common.js \
          res/js/status_utils.js

BOWER_DIR=third_party/bower_components

include ../kube/kube.mk

.PHONY: all
all: clean_webtools status

# Build the web assets in production mode.
.PHONY: web
web: webpack touch_elements core_js elements_html
	ln -sf ../../res res/imp/res
	npx webpack --mode=production

.PHONY: touch_elements
touch_elements:
	rm -f res/js/core.js
	touch elements.html

.PHONY: serve
serve:
	npm ci
	npx webpack-dev-server --mode=development --watch-poll

testgo:
	go test ./go/... -v

testjs:
	npx webpack --mode=development
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

status: web
	CGO_ENABLED=0 GOOS=linux go install -a ./go/status

release: status
	./build_release

run_docker: release
	docker run status

push: release pushk
	pushk --message="$(MESSAGE)" --use-temp-checkout status

# Build debug versions of core.js and elements.html.
.PHONY: debug
debug: clean_webtools debug_core_js debug_elements_html

include ../webtools/webtools.mk
include webpack/webpack.mk

.PHONY: build-frontend-ci
build-frontend-ci:
	# The below commands use //status/webpack/webpack.config.ts to build the code in //status/webpack.
	# In the future this should probably be updated to target //status/webpack.config.ts instead.
	cd webpack && npm ci
	cd webpack && npx webpack --mode=production
