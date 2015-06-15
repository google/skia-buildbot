.PHONY: build
build:
	echo

.PHONY: test
test: node_modules/lastupdate
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" \
		node_modules/karma/bin/karma start karma.conf.js

.PHONY: all
all: build

.PHONY: tags
tags:
	-rm tags
	find . -name "*.js" | xargs ctags --append

include ../webtools/webtools.mk
