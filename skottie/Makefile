default: app
	npx webpack --mode=development

release: app
	docker pull gcr.io/skia-public/skia-release:prod
	npx webpack --mode=production
	./build_release

serve: app
	npx webpack-dev-server --mode=development --watch

watch: app
	npx webpack --mode=development --watch

app: package-lock.json
	go install ./go/skottie

package-lock.json: package.json
	npm install
	touch package-lock.json

push: release skia-public
	pushk skottie

include ../make/clusters.mk
