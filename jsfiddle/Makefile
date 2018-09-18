build: core
	npx webpack --mode=development

serve: package-lock.json
	npx webpack-dev-server --watch-poll --mode=development

release_build: core
	npx webpack --mode=production

watch: core
	npx webpack --watch --mode=development

package-lock.json: package.json
	npm install

.PHONY: build serve release_build watch
