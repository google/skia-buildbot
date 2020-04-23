core:
	go install -v ./go/demosfe

serve:
	npx webpack-dev-server --watch-poll --mode-development

build_resources:
	npx webpack --mode=production

build: build_resources core

run: build
	demosfe --alsologtostderr

release: build
	./build_release
