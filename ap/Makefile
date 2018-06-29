default: package-lock.json
	npx webpack --mode=development

release: package-lock.json
	npx webpack --mode=production

serve: package-lock.json
	npx webpack-dev-server --mode=production --content-base ./dist --watch-poll

test:
	# Run the generated tests just once under Xvfb.
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

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

docs: package-lock.json
	npx jsdoc -c jsdoc.config.js

icons: package-lock.json
	go run generate_icons.go

package-lock.json: package.json
	npm install
