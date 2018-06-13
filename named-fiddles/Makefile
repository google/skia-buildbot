default: package-lock.json
	npx webpack --mode=development

release: package-lock.json
	npx webpack --mode=production

serve: package-lock.json
	npx webpack-dev-server --mode=development --watch-poll

watch: package-lock.json
	npx webpack --mode=development --watch-poll

package-lock.json: package.json
	npm install
