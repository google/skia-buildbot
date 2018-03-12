default:
	yarn
	npx webpack

release:
	npx webpack --config webpack.prod.js --define process.env.NODE_ENV=production

serve:
	npx webpack-dev-server --content-base ./dist --watch

publish:
	cd skia-elements; npm publish

update-major:
	cd skia-elements; npm version major
	echo "Don't forget to publish."

update-minor:
	cd skia-elements; npm version minor
	echo "Don't forget to publish."

update-patch:
	cd skia-elements; npm version patch
	echo "Don't forget to publish."

docs:
	npx jsdoc -c jsdoc.config.js
