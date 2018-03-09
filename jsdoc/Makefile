build:
	go install ./go/jsdocserver

docs:
	yarn
	npx jsdoc -c jsdoc.config.js `find ../ap/skia-elements ../common/modules -name "*.js"`
