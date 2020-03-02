#Use Vulcanize 1.0
VULCANIZE1=true

# The list of files we want to go into core.js, which is concat'd and
# minified. These files should be either present in the project, brought
# into third_party/bower_compoents via bower, or in node_modules.
BOWER_DIR=third_party/bower_components
CORE_SOURCE_FILES = node_modules/native-promise-only/npo.js \
          $(BOWER_DIR)/webcomponentsjs/webcomponents.min.js \
          $(BOWER_DIR)/page/page.js \
          $(BOWER_DIR)/d3/d3.js \
          ../../res/js/common.js \
          res/js/gold.js

.PHONY: web
web: touch_elements core_js elements_html

# Build debug versions of core.js and elements.html.
.PHONY: debug
debug: clean_webtools debug_core_js debug_elements_html

node_modules: package.json
	npm ci

.PHONY: touch_elements
touch_elements:
	rm -f res/js/core.js
	touch elements.html

include ../../webtools/webtools.mk
