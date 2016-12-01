PROJECT=skia-swarming

default:
	npm install

zip:
	rm -f $(PROJECT).zip
	zip $(PROJECT) *.js *.png *.html *.css manifest.json

clean:
	rm $(PROJECT).zip
