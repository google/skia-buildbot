build:
	go install ./go/jsdocserver

docs:
	yarn
	npx jsdoc -c jsdoc.config.js `find ../ap/skia-elements ../common/modules ./jsdoc/README.md`

push: build
	./build_release "`git log -n1 --format=%s`"
	go install -v ../push/go/pushcli
	pushcli jsdocserver skia-jsdoc

.PHONY: push docs build
