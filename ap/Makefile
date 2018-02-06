default:
	npx webpack

release:
	npx webpack --config webpack.prod.js --define process.env.NODE_ENV=production

serve:
	npx webpack-dev-server --content-base ./dist --watch

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

