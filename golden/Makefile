
# The list of files we want to go into core.js, which is concat'd and
# minified. These files should be either present in the project, brought
# into third_party/bower_compoents via bower, or in node_modules.
BOWER_DIR=third_party/bower_components
CORE_SOURCE_FILES = node_modules/native-promise-only/npo.js \
          $(BOWER_DIR)/webcomponentsjs/webcomponents.min.js \
          $(BOWER_DIR)/d3/d3.js \
          ../res/js/common.js

.PHONY: all
all: allgo touch_elements res/css/core.css core_js elements_html

include ../go/skiaversion/skiaversion.mk

.PHONY: skiacorrectness
skiacorrectness: skiaversion
	go install -v ./go/skiacorrectness

.PHONY: correctness_migratedb
correctness_migratedb: skiaversion
	go install -v ./go/correctness_migratedb

.PHONY: imagediff
imagediff:
	go install -v ./go/imagediff

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

.PHONY: touch_elements
touch_elements:
	rm -f res/js/core.js
	touch elements.html

.PHONY: release
release: all
	./build_release "$(MESSAGE)" "$(INSTANCE_ID)"

.PHONY: allgo
allgo: skiacorrectness correctness_migratedb imagediff

include ../webtools/webtools.mk
