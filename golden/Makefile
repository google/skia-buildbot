BOWER_DIR=app/third_party/bower_components

# The list of files we want to go into core.js, which is concat'd and
# minified. These files should be either present in the project, brought
# into third_party/bower_compoents via bower, or in node_modules.
CORE_SOURCE_FILES = node_modules/native-promise-only/npo.js \
          $(BOWER_DIR)/webcomponentsjs/webcomponents.min.js \
          ../res/js/common.js

BOWER_DIR=app/third_party/bower_components

GOLD_CSS_SOURCE_FILES = app/third_party/bower_components/bootstrap/dist/css/bootstrap.min.css \
                        app/css/app.css

GOLD_JS_SOURCE_FILES = app/third_party/bower_components/jquery/dist/jquery.min.js \
                       app/third_party/bower_components/flot/jquery.flot.js \
                       app/third_party/bower_components/angular/angular.js \
                       app/third_party/bower_components/angular-route/angular-route.js
GOLD_SRC_FONTS = app/third_party/bower_components/bootstrap/fonts

include ../go/skiaversion/skiaversion.mk

.PHONY: skiacorrectness
skiacorrectness: res/css/core.css core_js elements_html skiaversion
	go install -v ./go/skiacorrectness

.PHONY: correctness_migratedb
correctness_migratedb: skiaversion
	go install -v ./go/correctness_migratedb

.PHONY: packages
packages:
	go build -v ./go/...

# This is the quick version triggered by the presubmit script.
.PHONY: testgo
testgo:
	go test ./go/... -short

.PHONY: bench
bench:
	go test ./go/... -bench=. -run=NONE

.PHONY: test
test:
	go test ./go/...

# Build debug versions of core.js and elements.html.
.PHONY: debug
debug: clean_webtools debug_core_js debug_elements_html

# Build serving CSS from input CSS.
res/css/core.css: node_modules/lastupdate res/css/main.css
	./node_modules/.bin/autoprefixer res/css/main.css -o res/css/core.css

node_modules: package.json
	npm install

.PHONY: web
web:
	awk 'FNR==1{print ""}{print}' $(GOLD_CSS_SOURCE_FILES) > app/css/generated.css
	awk 'FNR==1{print ""}{print}' $(GOLD_JS_SOURCE_FILES) > app/js/generated.js
	-mkdir -p app/fonts
	cp $(GOLD_SRC_FONTS)/* app/fonts/

.PHONY: release
release: web skiacorrectness correctness_migratedb
	./build_release "$(MESSAGE)"

.PHONY: all
all: skiacorrectness correctness_migratedb

include ../webtools/webtools.mk
