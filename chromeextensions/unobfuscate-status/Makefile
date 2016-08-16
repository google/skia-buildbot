PROJECT=unobfuscate-status-page

default:
	npm install

zip: default
	rm -f $(PROJECT).zip
	zip -r $(PROJECT) node_modules *.js *.png *.html *.css manifest.json

clean:
	rm $(PROJECT).zip
	rm -R node_modules
