default: package-lock.json
	npx webpack --mode=development

release: package-lock.json
	npx webpack --mode=production

serve: package-lock.json
	npx webpack-dev-server --mode=development --content-base ./dist --watch-poll

test:
	# Run the generated tests just once under Xvfb.
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

docs: package-lock.json
	npx jsdoc -c jsdoc.config.js

package-lock.json: package.json
	npm install
