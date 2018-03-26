.PHONY: build
build:
	npm install

node_modules/karma/bin/karma:
	npm install

.PHONY: test
test: node_modules/karma/bin/karma
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" \
		npx karma start karma.conf.js

.PHONY: debug
debug: node_modules/karma/bin/karma
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" \
		npx karma --no-single-run start karma.conf.js

.PHONY: all
all: build

.PHONY: tags
tags:
	-rm tags
	find . -name "*.js" | xargs ctags --append

publish:
	npm publish

update-major:
	npm version major
	echo "Don't forget to publish."

update-minor:
	npm version minor
	echo "Don't forget to publish."

update-patch:
	npm version patch
	echo "Don't forget to publish."

