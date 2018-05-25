default: package-lock.json
	npx webpack --mode=development

release:
	npx webpack --mode=production

serve:
	npx webpack-dev-server --content-base ./dist --watch-poll

publish:
	cd elements-sk; npm publish

update-major:
	cd elements-sk; npm version major
	echo "Don't forget to publish."

update-minor:
	cd elements-sk; npm version minor
	echo "Don't forget to publish."

update-patch:
	cd elements-sk; npm version patch
	echo "Don't forget to publish."

docs:
	npx jsdoc -c jsdoc.config.js

package-lock.json: package.json
	npm install
