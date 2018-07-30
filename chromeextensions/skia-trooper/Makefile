PROJECT=skia-trooper

default:
	npm install

zip: default
	rm -f $(PROJECT).zip
	zip $(PROJECT) node_modules *.js *.png *.html manifest.json

clean:
	rm $(PROJECT).zip
	rm -R node_modules
