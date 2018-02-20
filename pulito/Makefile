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

zip:
	-rm -rf ./example/dist
	-rm -rf ./example/node_modules
	-rm -rf ./example/yarn.lock
	-rm skeleton.zip
	cd example; zip -r ../skeleton.zip .

