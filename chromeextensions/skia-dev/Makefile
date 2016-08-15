PROJECT=skia-dev

default:
	npm install

zip: default
	rm -f $(PROJECT).zip
	zip $(PROJECT) *.js *.png *.html *.css manifest.json

clean:
	rm $(PROJECT).zip
	rm -R node_modules
